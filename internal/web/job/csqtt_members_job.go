package job

import (
	"context"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type CsqttMembersJob struct{}

func NewCsqttMembersJob() *CsqttMembersJob { return new(CsqttMembersJob) }

func (j *CsqttMembersJob) Run() {
	if err := service.GetCsqttMemberService().SyncAll(context.Background()); err != nil {
		logger.Warning("csqtt members job:", err)
	}
}
