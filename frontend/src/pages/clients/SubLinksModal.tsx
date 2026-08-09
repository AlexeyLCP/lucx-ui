import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal, Table, Tooltip, Typography, message } from 'antd';
import type { TableColumnType } from 'antd';
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons';

import type { ClientRecord } from '@/hooks/useClients';
import { buildSubLinks, type SubSettingsLinks } from '@/lib/sub/links';

interface SubLinksModalProps {
  open: boolean;
  emails: string[];
  clients: ClientRecord[];
  subSettings?: SubSettingsLinks;
  onOpenChange: (open: boolean) => void;
}

interface Row {
  key: string;
  email: string;
  subId: string;
  link: string;
  jsonLink: string;
  clashLink: string;
  amneziaLink: string;
}

export default function SubLinksModal({
  open,
  emails,
  clients,
  subSettings,
  onOpenChange,
}: SubLinksModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();

  const rows = useMemo<Row[]>(() => {
    const byEmail = new Map(clients.map((c) => [c.email, c]));
    const out: Row[] = [];
    for (const email of emails) {
      const c = byEmail.get(email);
      if (!c?.subId) continue;
      const L = buildSubLinks(subSettings, c.subId);
      if (!L.sub && !L.json && !L.clash && !L.amnezia) continue;
      out.push({
        key: email,
        email,
        subId: c.subId,
        link: L.sub,
        jsonLink: L.json,
        clashLink: L.clash,
        amneziaLink: L.amnezia,
      });
    }
    return out;
  }, [emails, clients, subSettings]);

  const allText = useMemo(
    () => rows.map((r) => [r.email, r.link, r.jsonLink, r.clashLink, r.amneziaLink].filter(Boolean).join('\t')).join('\n'),
    [rows],
  );

  async function copy(text: string, label?: string) {
    try {
      await navigator.clipboard.writeText(text);
      messageApi.success(label || t('copied'));
    } catch {
      messageApi.error(t('somethingWentWrong'));
    }
  }

  function download() {
    const blob = new Blob([allText], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    a.href = url;
    a.download = `sub-links-${stamp}.txt`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  const columns: TableColumnType<Row>[] = [
    {
      title: t('pages.clients.client'),
      dataIndex: 'email',
      key: 'email',
      width: 160,
      ellipsis: true,
    },
  ];

  function addLinkCol(title: string, dataIndex: keyof Row, width = 200) {
    columns.push({
      title,
      dataIndex,
      key: dataIndex,
      ellipsis: true,
      width,
      render: (link: string) => link ? (
        <Tooltip title={link} placement="topLeft">
          <Typography.Text copyable={false} ellipsis>{link}</Typography.Text>
        </Tooltip>
      ) : '—',
    });
    columns.push({
      title: '',
      key: `${dataIndex}-copy`,
      width: 48,
      render: (_v, row) => {
        const link = row[dataIndex] as string;
        if (!link) return null;
        return <Button size="small" type="text" aria-label={t('copy')} icon={<CopyOutlined />} onClick={() => copy(link, t('copied'))} />;
      },
    });
  }

  if (rows.some((r) => r.link)) addLinkCol(t('pages.clients.subLinkColumn'), 'link');
  if (rows.some((r) => r.jsonLink)) addLinkCol(t('pages.clients.subJsonLinkColumn'), 'jsonLink');
  if (rows.some((r) => r.clashLink)) addLinkCol('Clash', 'clashLink');
  if (rows.some((r) => r.amneziaLink)) addLinkCol('Amnezia', 'amneziaLink');

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.clients.subLinksTitle', { count: rows.length })}
        width={960}
        onCancel={() => onOpenChange(false)}
        footer={[
          <Button key="dl" icon={<DownloadOutlined />} onClick={download} disabled={!rows.length}>
            {t('download')}
          </Button>,
          <Button
            key="copy"
            type="primary"
            icon={<CopyOutlined />}
            onClick={() => copy(allText, t('pages.clients.subLinksCopiedAll', { count: rows.length }))}
            disabled={!rows.length}
          >
            {t('pages.clients.subLinksCopyAll')}
          </Button>,
        ]}
      >
        {!subSettings?.enable && !subSettings?.subAwgEnable && !subSettings?.subClashEnable && !subSettings?.subJsonEnable ? (
          <Alert
            type="warning"
            showIcon
            title={t('pages.clients.subLinksDisabled')}
            description={t('pages.clients.subLinksDisabledHint')}
          />
        ) : !rows.length ? (
          <Alert type="info" showIcon title={t('pages.clients.subLinksEmpty')} />
        ) : (
          <Table<Row>
            size="small"
            rowKey="key"
            columns={columns}
            dataSource={rows}
            pagination={false}
            scroll={{ x: true }}
          />
        )}
      </Modal>
    </>
  );
}
