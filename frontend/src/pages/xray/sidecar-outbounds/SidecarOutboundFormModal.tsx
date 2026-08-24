// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, Modal, Select, Switch, message } from 'antd';

import { sidecarOutboundsApi } from '@/api/sidecar-outbounds';
import type { SidecarOutbound, SidecarProtocol } from '@/schemas/sidecar-outbound';

interface Props {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  initial?: SidecarOutbound | null;
}

export function SidecarOutboundFormModal({ open, onClose, onSaved, initial }: Props) {
  const { t } = useTranslation();
  const [protocol, setProtocol] = useState<SidecarProtocol>('naive');
  const [tag, setTag] = useState('');
  const [remark, setRemark] = useState('');
  const [enable, setEnable] = useState(true);
  const [link, setLink] = useState('');
  const [host, setHost] = useState('');
  const [port, setPort] = useState('');
  const [user, setUser] = useState('');
  const [pass, setPass] = useState('');
  const [saving, setSaving] = useState(false);
  const [messageApi, holder] = message.useMessage();

  useEffect(() => {
    if (!open) return;
    if (initial) {
      setProtocol(initial.protocol);
      setTag(initial.tag);
      setRemark(initial.remark ?? '');
      setEnable(initial.enable);
      try {
        const s = JSON.parse(initial.settings || '{}') as Record<string, unknown>;
        setLink(String(s.link ?? ''));
        setHost(String(s.host ?? ''));
        setPort(s.port ? String(s.port) : '');
        setUser(String(s.user ?? ''));
        setPass(String(s.pass ?? ''));
      } catch {
        setLink('');
        setHost('');
        setPort('');
        setUser('');
        setPass('');
      }
      return;
    }
    setProtocol('naive');
    setTag('');
    setRemark('');
    setEnable(true);
    setLink('');
    setHost('');
    setPort('');
    setUser('');
    setPass('');
  }, [open, initial]);

  async function parseLink() {
    try {
      const res = await sidecarOutboundsApi.parseLink(link);
      if (!res.success || !res.obj) {
        messageApi.error(res.msg || t('pages.xray.sidecarOutbound.parseFailed'));
        return;
      }
      setProtocol(res.obj.protocol);
      setHost(res.obj.settings.host);
      setPort(res.obj.settings.port ? String(res.obj.settings.port) : '');
      setUser(res.obj.settings.user);
      setPass(res.obj.settings.pass ?? '');
      messageApi.success(t('pages.xray.sidecarOutbound.parsed'));
    } catch (e) {
      messageApi.error((e as Error)?.message || t('pages.xray.sidecarOutbound.parseFailed'));
    }
  }

  async function save() {
    setSaving(true);
    try {
      const settings = JSON.stringify({
        link,
        host,
        port: Number(port) || 0,
        user,
        pass,
      });
      const payload = {
        protocol,
        tag,
        remark,
        enable,
        settings,
      };
      if (initial) {
        await sidecarOutboundsApi.update({ ...initial, ...payload });
      } else {
        await sidecarOutboundsApi.add(payload);
      }
      messageApi.success(t('pages.xray.sidecarOutbound.saved'));
      onSaved();
      onClose();
    } catch (e) {
      messageApi.error((e as Error)?.message || 'failed');
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      title={initial ? t('pages.xray.sidecarOutbound.edit') : t('pages.xray.sidecarOutbound.add')}
      footer={[
        <Button key="cancel" onClick={onClose}>
          {t('cancel')}
        </Button>,
        <Button key="ok" type="primary" loading={saving} onClick={() => void save()}>
          {t('save')}
        </Button>,
      ]}
    >
      {holder}
      <Form layout="vertical">
        <Form.Item label={t('pages.xray.sidecarOutbound.protocol')}>
          <Select
            value={protocol}
            disabled={!!initial}
            onChange={(v) => setProtocol(v)}
            options={[
              { value: 'naive', label: t('pages.xray.sidecarOutbound.naive') },
              { value: 'mieru', label: t('pages.xray.sidecarOutbound.mieru') },
              { value: 'trusttunnel', label: t('pages.xray.sidecarOutbound.trusttunnel') },
            ]}
          />
        </Form.Item>
        <Form.Item label={t('pages.xray.sidecarOutbound.pasteLink')}>
          <Input.TextArea
            value={link}
            onChange={(e) => setLink(e.target.value)}
            rows={2}
            placeholder="naive+https://… / mierus://… / tt://…"
          />
          <Button style={{ marginTop: 8 }} onClick={() => void parseLink()}>
            {t('pages.xray.sidecarOutbound.parseAndFill')}
          </Button>
        </Form.Item>
        <Form.Item label={t('pages.xray.awgOutbound.tag')}>
          <Input value={tag} onChange={(e) => setTag(e.target.value)} placeholder="auto" />
        </Form.Item>
        <Form.Item label={t('pages.xray.awgOutbound.remark')}>
          <Input value={remark} onChange={(e) => setRemark(e.target.value)} />
        </Form.Item>
        <Form.Item label={t('pages.xray.awgOutbound.enable')}>
          <Switch checked={enable} onChange={setEnable} />
        </Form.Item>
        <Form.Item label="Host">
          <Input value={host} onChange={(e) => setHost(e.target.value)} />
        </Form.Item>
        <Form.Item label="Port">
          <Input value={port} onChange={(e) => setPort(e.target.value)} />
        </Form.Item>
        <Form.Item label="User">
          <Input value={user} onChange={(e) => setUser(e.target.value)} />
        </Form.Item>
        <Form.Item label="Password">
          <Input.Password value={pass} onChange={(e) => setPass(e.target.value)} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
