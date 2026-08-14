import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Form, Input, Modal, Select, Space, Switch, Tooltip, message } from 'antd';
import { PlusOutlined, MinusOutlined, QuestionCircleOutlined } from '@ant-design/icons';
import { FormProvider, useForm, useWatch } from 'react-hook-form';
import { InputAddon } from '@/components/ui';
import { GeoTokenInput } from '@/components/geodata';
import { FormField } from '@/components/form/rhf';
import { useInboundOptions } from '@/api/queries/useInboundOptions';
import { HttpUtil } from '@/utils';
import { RuleFormSchema, type RuleFormValues } from '@/schemas/xray';
import type { ClientRecord, InboundOption } from '@/schemas/client';
import { buildRemarkByTag, formatInboundTag, isApiRule } from './helpers';

export interface RoutingRule {
  enabled?: boolean;
  type?: string;
  domain?: string | string[];
  ip?: string | string[];
  port?: string;
  sourcePort?: string;
  vlessRoute?: string;
  network?: string;
  sourceIP?: string | string[];
  user?: string | string[];
  inboundTag?: string[];
  protocol?: string[];
  attrs?: Record<string, string>;
  outboundTag?: string;
  balancerTag?: string;
  [key: string]: unknown;
}

interface RuleFormModalProps {
  open: boolean;
  rule: RoutingRule | null;
  inboundTags: string[];
  outboundTags: string[];
  balancerTags: string[];
  onClose: () => void;
  onConfirm: (rule: Record<string, unknown>) => void;
}

const initialForm = (): RuleFormValues => ({
  enabled: true,
  domain: '',
  ip: '',
  port: '',
  sourcePort: '',
  vlessRoute: '',
  network: '',
  sourceIP: '',
  user: '',
  inboundTag: [],
  protocol: [],
  attrs: [],
  outboundTag: '',
  balancerTag: '',
});

const NETWORKS = ['', 'tcp', 'udp', 'tcp,udp'];
const PROTOCOLS = ['http', 'tls', 'bittorrent', 'quic'];

function csv(value: string): string[] {
  if (!value) return [];
  return value.split(',').map((s) => s.trim()).filter(Boolean);
}

/** Single-host tunnel IPs from AWG/WG allowedIPs CSV (skip 0.0.0.0/0 etc.). */
function singleHostIps(allowedIPs: string | undefined): string[] {
  if (!allowedIPs) return [];
  const out: string[] = [];
  for (const raw of allowedIPs.split(',').map((s) => s.trim()).filter(Boolean)) {
    if (raw === '0.0.0.0/0' || raw === '::/0') continue;
    const bare = raw.includes('/') ? raw.split('/')[0] : raw;
    const bits = raw.includes('/') ? Number(raw.split('/')[1]) : NaN;
    if (bare.includes(':')) {
      if (!Number.isNaN(bits) && bits !== 128) continue;
    } else if (!Number.isNaN(bits) && bits !== 32) {
      continue;
    }
    if (bare) out.push(raw.includes('/') ? raw : `${bare}/32`);
  }
  return out;
}

function clientProtocols(c: ClientRecord, byId: Map<number, InboundOption>): Set<string> {
  const s = new Set<string>();
  for (const id of c.inboundIds || []) {
    const p = byId.get(id)?.protocol;
    if (p) s.add(p);
  }
  return s;
}

type ClientOpt = { value: string; label: string; kind: 'user' | 'source' | 'inbound'; token: string };

function buildClientOptions(clients: ClientRecord[], inbounds: InboundOption[]): ClientOpt[] {
  const byId = new Map(inbounds.map((i) => [i.id, i]));
  const opts: ClientOpt[] = [];
  for (const c of clients) {
    if (!c.email) continue;
    const protos = clientProtocols(c, byId);
    const isAwg = protos.has('awg');
    const isWg = protos.has('wireguard');
    const isNaive = protos.has('naive');
    if (isAwg || isWg) {
      const ips = singleHostIps(c.allowedIPs);
      if (ips.length === 0) continue;
      const proto = isAwg ? 'AWG' : 'WG';
      for (const ip of ips) {
        opts.push({
          value: `src:${ip}`,
          kind: 'source',
          token: ip,
          label: `${c.email} · ${proto} · ${ip}`,
        });
      }
    }
    // NaiveProxy → Xray SOCKS bridge is tagged with the inbound's tag (not
    // per-user email). Picking a Naive client fills inboundTag so operators
    // can scatter users by attaching them to different Naive inbounds.
    if (isNaive) {
      for (const id of c.inboundIds || []) {
        const ib = byId.get(id);
        if (!ib || ib.protocol !== 'naive' || !ib.tag) continue;
        opts.push({
          value: `in:${ib.tag}`,
          kind: 'inbound',
          token: ib.tag,
          label: `${c.email} · Naive · ${ib.tag}`,
        });
      }
    }
    // Xray-auth protocols keep email → routing.user
    if (!isAwg || protos.size > 1) {
      const hasXrayUser = [...protos].some(
        (p) => p !== 'awg' && p !== 'wireguard' && p !== 'tun' && p !== 'tunnel' && p !== 'naive' && p !== 'mtproto' && p !== 'mieru' && p !== 'trusttunnel',
      );
      if (hasXrayUser || (!isAwg && !isWg && !isNaive && !protos.has('mtproto') && !protos.has('mieru') && !protos.has('trusttunnel'))) {
        const protoLabel =
          [...protos].filter((p) => p !== 'awg' && p !== 'wireguard' && p !== 'naive' && p !== 'mieru' && p !== 'trusttunnel').join('/') || 'xray';
        opts.push({
          value: `user:${c.email}`,
          kind: 'user',
          token: c.email,
          label: `${c.email} · ${protoLabel}`,
        });
      }
    }
  }
  opts.sort((a, b) => a.label.localeCompare(b.label));
  return opts;
}

export default function RuleFormModal({
  open,
  rule,
  inboundTags,
  outboundTags,
  balancerTags,
  onClose,
  onConfirm,
}: RuleFormModalProps) {
  const { t } = useTranslation();
  const methods = useForm<RuleFormValues>({ defaultValues: initialForm() });
  const isEdit = rule != null;
  const [clientPick, setClientPick] = useState<string[]>([]);

  const { data: inboundOptions } = useInboundOptions();
  const inboundList = useMemo(() => inboundOptions ?? [], [inboundOptions]);
  const remarkByTag = useMemo(() => buildRemarkByTag(inboundList), [inboundList]);

  const { data: clients = [] } = useQuery({
    queryKey: ['routing', 'clientPickList'],
    queryFn: async () => {
      const msg = await HttpUtil.get<ClientRecord[]>('/panel/api/clients/list', undefined, { silent: true });
      return (msg?.success && Array.isArray(msg.obj) ? msg.obj : []) as ClientRecord[];
    },
    enabled: open,
    staleTime: 30_000,
  });

  const clientOptions = useMemo(
    () => buildClientOptions(clients, inboundList),
    [clients, inboundList],
  );

  const clientOptByValue = useMemo(() => {
    const m = new Map<string, ClientOpt>();
    for (const o of clientOptions) m.set(o.value, o);
    return m;
  }, [clientOptions]);

  useEffect(() => {
    if (!open) return;
    if (rule) {
      const sourceIP = Array.isArray(rule.sourceIP) ? rule.sourceIP.join(',') : rule.sourceIP || '';
      const user = Array.isArray(rule.user) ? rule.user.join(',') : rule.user || '';
      methods.reset({
        enabled: rule.enabled !== false,
        domain: Array.isArray(rule.domain) ? rule.domain.join(',') : rule.domain || '',
        ip: Array.isArray(rule.ip) ? rule.ip.join(',') : rule.ip || '',
        port: rule.port || '',
        sourcePort: rule.sourcePort || '',
        vlessRoute: rule.vlessRoute || '',
        network: rule.network || '',
        sourceIP,
        user,
        inboundTag: rule.inboundTag || [],
        protocol: rule.protocol || [],
        attrs: rule.attrs ? Object.entries(rule.attrs) : [],
        outboundTag: rule.outboundTag || '',
        balancerTag: rule.balancerTag || '',
      });
      const pick: string[] = [];
      for (const u of csv(user)) pick.push(`user:${u}`);
      for (const ip of csv(sourceIP)) {
        pick.push(ip.includes('/') || ip.includes(':') ? `src:${ip}` : `src:${ip}/32`);
      }
      for (const tag of rule.inboundTag || []) {
        if (tag) pick.push(`in:${tag}`);
      }
      setClientPick(pick);
    } else {
      methods.reset(initialForm());
      setClientPick([]);
    }
    // `methods` is deliberately not a dependency: react-hook-form hands back a
    // fresh object every render, so listing it re-runs this reset on every
    // render and hangs the modal. The effect only ever wants to fire when the
    // modal opens or the rule being edited changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, rule]);

  const attrs = useWatch({ control: methods.control, name: 'attrs' }) ?? [];

  function applyClientPick(values: string[]) {
    setClientPick(values);
    const users: string[] = [];
    const sources: string[] = [];
    const inboundFromPick: string[] = [];
    for (const v of values) {
      const opt = clientOptByValue.get(v);
      if (opt) {
        if (opt.kind === 'user') users.push(opt.token);
        else if (opt.kind === 'inbound') inboundFromPick.push(opt.token);
        else sources.push(opt.token);
        continue;
      }
      // free-typed tag: email → user, in:tag → inbound, otherwise sourceIP
      if (v.startsWith('in:')) {
        inboundFromPick.push(v.slice(3));
      } else if (v.includes('@') || v.startsWith('user:')) {
        users.push(v.replace(/^user:/, ''));
      } else {
        sources.push(v.replace(/^src:/, ''));
      }
    }
    methods.setValue('user', users.join(','), { shouldDirty: true });
    methods.setValue('sourceIP', sources.join(','), { shouldDirty: true });
    // Merge Naive inbound tags from the client picker with any tags the
    // operator already chose in the inboundTag Select (do not wipe them).
    const clientInboundTokens = new Set(
      clientOptions.filter((o) => o.kind === 'inbound').map((o) => o.token),
    );
    const existing = (methods.getValues('inboundTag') as string[] | undefined) || [];
    const kept = existing.filter((t) => t && !clientInboundTokens.has(t));
    const merged = [...new Set([...kept, ...inboundFromPick.filter(Boolean)])];
    methods.setValue('inboundTag', merged, { shouldDirty: true });
  }

  function submit() {
    const validated = RuleFormSchema.safeParse(methods.getValues());
    if (!validated.success) return;
    const v = validated.data;
    const built: Record<string, unknown> = {
      type: 'field',
      enabled: v.enabled,
      domain: csv(v.domain),
      ip: csv(v.ip),
      port: v.port,
      sourcePort: v.sourcePort,
      vlessRoute: v.vlessRoute,
      network: v.network,
      sourceIP: csv(v.sourceIP),
      user: csv(v.user),
      inboundTag: v.inboundTag,
      protocol: v.protocol,
      attrs: Object.fromEntries(v.attrs.filter(([k]) => k)),
      outboundTag: v.outboundTag === '' ? undefined : v.outboundTag,
      balancerTag: v.balancerTag === '' ? undefined : v.balancerTag,
    };
    const managedKeys = new Set(Object.keys(built));
    const out: Record<string, unknown> = {};
    if (rule) {
      for (const [key, value] of Object.entries(rule)) {
        if (!managedKeys.has(key) && value !== undefined) out[key] = value;
      }
    }
    for (const [k, v] of Object.entries(built)) {
      if (v == null) continue;
      if (Array.isArray(v) && v.length === 0) continue;
      if (typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length === 0) continue;
      if (v === '') continue;
      out[k] = v;
    }
    // Xray rejects a field rule with no effective matchers ("this rule has no
    // effective fields") and fails to start, taking the whole panel down.
    // A rule MUST carry at least one of: domain, ip, port, sourcePort, network,
    // sourceIP, user, inboundTag, protocol, attrs — otherwise the outboundTag/
    // balancerTag targets nothing. Guard here so the user never saves an
    // invalid rule (matches AGENTS.md Debug Pattern 5).
    // "vlessRoute" alone is also a matcher (Xray accepts it), so include it.
    const matchers = ['domain', 'ip', 'port', 'sourcePort', 'network', 'sourceIP', 'user', 'inboundTag', 'protocol', 'attrs', 'vlessRoute'];
    const hasMatcher = matchers.some((k) => {
      const val = (out as Record<string, unknown>)[k];
      if (val == null || val === '') return false;
      if (Array.isArray(val)) return val.length > 0;
      if (typeof val === 'object') return Object.keys(val).length > 0;
      return true;
    });
    // Block only when creating: a brand-new empty rule is always the footgun
    // above. On edit, the rule already lives in the config (it may predate this
    // guard, come from a template, or be edited for an unrelated field such as
    // ruleTag), so blocking would trap the user in a modal they cannot save.
    // Warn instead — same information, no dead end.
    if (!hasMatcher) {
      if (!isEdit) {
        message.error(t('pages.xray.ruleForm.noMatcherError'));
        return;
      }
      message.warning(t('pages.xray.ruleForm.noMatcherError'));
    }
    // outboundTag OR balancerTag is also required — a rule with matchers but
    // no target is silently a no-op in Xray. Warn (don't block) so advanced
    // users can still save a half-built rule, but surface the issue clearly.
    if (!out.outboundTag && !out.balancerTag) {
      message.warning(t('pages.xray.ruleForm.noTargetWarning'));
    }
    onConfirm(out);
  }

  // applyMatchAllPreset fills the rule as a catch-all: matches every domain
  // via "domain:" (Xray's "any domain" wildcard). The user still has to pick
  // an outboundTag/balancerTag before saving — submit() will warn if not.
  // Useful for "send all traffic through VPN" rules without hand-typing
  // regex/wildcard syntax (the most common footgun that produced empty rules).
  function applyMatchAllPreset() {
    methods.setValue('domain', 'domain:');
    message.info(t('pages.xray.ruleForm.matchAllApplied'));
  }

  const title = isEdit
    ? `${t('edit')} ${t('pages.xray.Routings')}`
    : `+ ${t('pages.xray.Routings')}`;
  const okText = isEdit ? t('pages.clients.submitEdit') : t('create');

  return (
    <Modal
      open={open}
      title={title}
      okText={okText}
      cancelText={t('close')}
      mask={{ closable: false }}
      width={640}
      onOk={submit}
      onCancel={onClose}
    >
      <FormProvider {...methods}>
        <Form colon={false} labelCol={{ md: { span: 8 } }} wrapperCol={{ md: { span: 14 } }}>
          <FormField name="enabled" label={t('enable')} valueProp="checked">
            <Switch disabled={isApiRule(rule ?? {})} />
          </FormField>

          {/* LUCX-HOOK: AWG outbound — routing-rule validation UX.
              Xray rejects a field rule with no effective matchers and fails to
              start (AGENTS.md Pattern 5). A common footgun was creating a rule
              with only outboundTag set ("send all traffic through VPN") and no
              domain/ip/inboundTag. Offer a one-click "Match all traffic" preset
              so the user doesn't have to know Xray's "domain:" wildcard syntax,
              and guard submit() against saving an invalid rule. */}
          <Form.Item label=" " colon={false}>
            <Space direction="vertical" style={{ width: '100%' }}>
              <Button
                size="small"
                type="dashed"
                icon={<PlusOutlined />}
                onClick={applyMatchAllPreset}
              >
                {t('pages.xray.ruleForm.matchAllPreset')}
              </Button>
            </Space>
          </Form.Item>
          {/* END LUCX-HOOK */}

          <FormField
            name="sourceIP"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('pages.xray.ruleForm.sourceIps')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <GeoTokenInput kind="ip" placeholder="0.0.0.0/8, fc00::/7, geoip:ir" />
          </FormField>

          <FormField
            name="sourcePort"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('pages.xray.ruleForm.sourcePort')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Input placeholder="53,443,1000-2000" />
          </FormField>

          <FormField
            name="vlessRoute"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('pages.xray.ruleForm.vlessRoute')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Input placeholder="53,443,1000-2000" />
          </FormField>

          <FormField name="network" label={t('pages.inbounds.network')}>
            <Select options={NETWORKS.map((n) => ({ value: n, label: n || '(any)' }))} />
          </FormField>

          <FormField name="protocol" label={t('pages.inbounds.protocol')}>
            <Select mode="multiple" options={PROTOCOLS.map((p) => ({ value: p, label: p }))} />
          </FormField>

          <Form.Item label={t('pages.xray.ruleForm.attributes')}>
            <Button
              size="small"
              aria-label={t('add')}
              icon={<PlusOutlined />}
              onClick={() => methods.setValue('attrs', [...attrs, ['', ''] as [string, string]])}
            />
          </Form.Item>
          <Form.Item wrapperCol={{ span: 24 }}>
            {attrs.map((attr, idx) => (
              <Space.Compact key={idx} block className="mb-8">
                <InputAddon>{`${idx + 1}`}</InputAddon>
                <Input
                  value={attr[0]}
                  aria-label={t('pages.nodes.name')}
                  placeholder={t('pages.nodes.name')}
                  onChange={(e) => {
                    const next = attrs.map((a, i) => (i === idx ? ([e.target.value, a[1]] as [string, string]) : a));
                    methods.setValue('attrs', next);
                  }}
                />
                <Input
                  value={attr[1]}
                  aria-label={t('pages.xray.ruleForm.value')}
                  placeholder={t('pages.xray.ruleForm.value')}
                  onChange={(e) => {
                    const next = attrs.map((a, i) => (i === idx ? ([a[0], e.target.value] as [string, string]) : a));
                    methods.setValue('attrs', next);
                  }}
                />
                <Button
                  aria-label={t('remove')}
                  icon={<MinusOutlined />}
                  onClick={() => methods.setValue('attrs', attrs.filter((_, i) => i !== idx))}
                />
              </Space.Compact>
            ))}
          </Form.Item>

          <FormField
            name="ip"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                IP <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <GeoTokenInput kind="ip" placeholder="0.0.0.0/8, fc00::/7, geoip:ir" />
          </FormField>

          <FormField
            name="domain"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('domainName')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <GeoTokenInput kind="domain" placeholder="google.com, geosite:cn" />
          </FormField>

          <Form.Item
            label={
              <Tooltip title={t('pages.xray.ruleForm.clientPickHint')}>
                {t('pages.xray.ruleForm.user')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Select
              mode="tags"
              showSearch
              allowClear
              value={clientPick}
              onChange={applyClientPick}
              optionFilterProp="label"
              placeholder={t('pages.xray.ruleForm.clientPickPlaceholder')}
              options={clientOptions.map((o) => ({ value: o.value, label: o.label }))}
              filterOption={(input, option) =>
                String(option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            title={t('pages.xray.ruleForm.clientPickAwgNote')}
          />
          {/* Keep RHF fields in sync for submit (picker writes user + sourceIP). */}
          <FormField name="user" noStyle>
            <Input type="hidden" />
          </FormField>

          <FormField
            name="port"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('pages.inbounds.port')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Input placeholder="53,443,1000-2000" />
          </FormField>

          <FormField name="inboundTag" label={t('pages.xray.ruleForm.inboundTags')}>
            <Select
              mode="multiple"
              options={inboundTags.map((tag) => ({ value: tag, label: formatInboundTag(tag, remarkByTag) }))}
            />
          </FormField>

          <FormField name="outboundTag" label={t('pages.xray.ruleForm.outboundTag')}>
            <Select options={outboundTags.map((tag) => ({ value: tag, label: tag || '(none)' }))} />
          </FormField>

          <FormField
            name="balancerTag"
            label={
              <Tooltip title={t('pages.xray.ruleForm.balancerTagTooltip')}>
                {t('pages.xray.ruleForm.balancerTag')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Select options={balancerTags.map((tag) => ({ value: tag, label: tag || '(none)' }))} />
          </FormField>
        </Form>
      </FormProvider>
    </Modal>
  );
}
