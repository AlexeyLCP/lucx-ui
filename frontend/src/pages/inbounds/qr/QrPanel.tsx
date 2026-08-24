import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, QRCode, Spin, Tag, Tooltip, message } from 'antd';
import { CopyOutlined, DownloadOutlined, PictureOutlined } from '@ant-design/icons';

import { ClipboardManager, FileManager } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import { fetchSubscriptionBody, isAmneziaVpnUrl } from '@/lib/sub/fetchBody';
import './QrPanel.css';

interface QrPanelProps {
  value: string;
  remark?: string;
  downloadName?: string;
  size?: number;
  showQr?: boolean;
}

async function svgToPngBlob(svgEl: SVGSVGElement | null, size: number): Promise<Blob | null> {
  if (!svgEl) return null;
  const svgData = new XMLSerializer().serializeToString(svgEl);
  const svgBlob = new Blob([svgData], { type: 'image/svg+xml;charset=utf-8' });
  const url = URL.createObjectURL(svgBlob);
  return new Promise<Blob | null>((resolve) => {
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = size;
      canvas.height = size;
      const ctx = canvas.getContext('2d');
      if (!ctx) {
        URL.revokeObjectURL(url);
        resolve(null);
        return;
      }
      ctx.fillStyle = '#ffffff';
      ctx.fillRect(0, 0, size, size);
      ctx.drawImage(img, 0, 0, size, size);
      URL.revokeObjectURL(url);
      canvas.toBlob((blob) => resolve(blob), 'image/png');
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      resolve(null);
    };
    img.src = url;
  });
}

function downloadImageBlob(blob: Blob, remark: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `${remark || 'qrcode'}.png`;
  link.click();
  URL.revokeObjectURL(url);
}

export default function QrPanel({
  value,
  remark = '',
  downloadName = '',
  size = 360,
  showQr = true,
}: QrPanelProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const qrRef = useRef<HTMLDivElement | null>(null);
  // Resolve Amnezia ?format=vpn HTTPS URLs to the vpn:// body for QR/copy.
  const [resolved, setResolved] = useState(value);
  const [resolving, setResolving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    if (!isAmneziaVpnUrl(value)) {
      setResolved(value);
      setResolving(false);
      return;
    }
    setResolving(true);
    void fetchSubscriptionBody(value)
      .then((body) => {
        if (!cancelled) setResolved(body);
      })
      .catch(() => {
        if (!cancelled) setResolved(value);
      })
      .finally(() => {
        if (!cancelled) setResolving(false);
      });
    return () => {
      cancelled = true;
    };
  }, [value]);

  async function copy() {
    try {
      const text =
        isAmneziaVpnUrl(value) && !resolved.startsWith('vpn://')
          ? await fetchSubscriptionBody(value)
          : resolved;
      const ok = await ClipboardManager.copyText(text);
      if (ok) messageApi.success(t('copied'));
    } catch (e) {
      messageApi.error(e instanceof Error && e.message ? e.message : t('somethingWentWrong'));
    }
  }

  function download() {
    if (!downloadName) return;
    FileManager.downloadTextFile(resolved, downloadName);
  }

  async function copyImage() {
    const svgEl = qrRef.current?.querySelector('svg') as SVGSVGElement | null;
    const blob = await svgToPngBlob(svgEl, size);
    if (!blob) return;
    try {
      await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]);
      messageApi.success(t('copied'));
    } catch {
      downloadImageBlob(blob, remark);
    }
  }

  async function downloadImage() {
    const svgEl = qrRef.current?.querySelector('svg') as SVGSVGElement | null;
    const blob = await svgToPngBlob(svgEl, size);
    if (blob) downloadImageBlob(blob, remark);
  }

  return (
    <div className="qr-panel">
      {messageContextHolder}
      <div className="qr-panel-header">
        <Tag color="green" className="qr-remark">
          {remark}
        </Tag>
        <Tooltip title={t('copy')}>
          <Button size="small" icon={<CopyOutlined />} aria-label={t('copy')} onClick={copy} />
        </Tooltip>
        {showQr && (
          <Tooltip title={t('downloadImage')}>
            <Button
              size="small"
              icon={<PictureOutlined />}
              aria-label={t('downloadImage')}
              onClick={downloadImage}
            />
          </Tooltip>
        )}
        {downloadName && (
          <Tooltip title={t('download')}>
            <Button
              size="small"
              icon={<DownloadOutlined />}
              aria-label={t('download')}
              onClick={download}
            />
          </Tooltip>
        )}
      </div>
      {showQr && resolving && (
        <div className="qr-panel-canvas" style={{ padding: 24, textAlign: 'center' }}>
          <Spin />
        </div>
      )}
      {showQr && !resolving && resolved.length <= 2000 && (
        <div
          ref={qrRef}
          className="qr-panel-canvas"
          role="button"
          tabIndex={0}
          aria-label={t('copy')}
          onClick={copyImage}
          onKeyDown={(event) => activateOnKey(copyImage)(event)}
        >
          <Tooltip title={t('copy')}>
            <QRCode
              className="qr-code"
              value={resolved}
              size={size}
              type="svg"
              bordered={false}
              color="#000000"
              bgColor="#ffffff"
            />
          </Tooltip>
        </div>
      )}
      {showQr && !resolving && resolved.length > 2000 && (
        <div
          className="qr-panel-canvas"
          style={{ padding: 16, textAlign: 'center', color: 'var(--ant-color-text-tertiary)' }}
        >
          {t('qrTooLarge')}
        </div>
      )}
    </div>
  );
}
