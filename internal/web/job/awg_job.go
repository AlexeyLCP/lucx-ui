package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// AwgJob reconciles the running AWG kernel-interface sidecars against the
// enabled AWG inbounds in the database, restarts any that crashed, and folds
// the per-inbound traffic scraped from `awg show transfer` into the usual
// inbound traffic accounting. Mirrors MtprotoJob one-for-one.
type AwgJob struct {
	inboundService service.InboundService
}

// NewAwgJob creates a new AWG reconcile/traffic job instance.
func NewAwgJob() *AwgJob {
	return new(AwgJob)
}

// Run reconciles desired AWG inbounds with running interfaces and records
// traffic deltas.
func (j *AwgJob) Run() {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("awg job: get inbounds failed:", err)
		return
	}

	var desired []awg.Instance
	for _, ib := range inbounds {
		if ib.Protocol != model.AWG || !ib.Enable || ib.NodeID != nil {
			continue
		}
		if inst, ok := awg.InstanceFromInbound(ib); ok {
			desired = append(desired, inst)
		}
	}

	mgr := awg.GetManager()
	mgr.Reconcile(desired)

	deltas := mgr.CollectTraffic()
	if len(deltas) == 0 {
		return
	}
	traffics := make([]*xray.Traffic, 0, len(deltas))
	for _, d := range deltas {
		traffics = append(traffics, &xray.Traffic{
			IsInbound: true,
			Tag:       d.Tag,
			Up:        d.Up,
			Down:      d.Down,
		})
	}
	if _, _, err := j.inboundService.AddTraffic(traffics, nil); err != nil {
		logger.Warning("awg job: add traffic failed:", err)
	}
}