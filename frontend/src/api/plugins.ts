import { HttpUtil, type Msg } from '@/utils';

export type PluginStatus = {
  manifest: { id: string; name: string; version: string; type: string };
  installed: boolean;
  running: boolean;
  pid?: number;
  binaryPath: string;
  dataPath: string;
  logPath: string;
  lastError?: string;
};

export const pluginsApi = {
  list: (): Promise<Msg<PluginStatus[]>> => HttpUtil.get('/panel/api/plugins/list'),
  status: (id: string): Promise<Msg<PluginStatus>> =>
    HttpUtil.get(`/panel/api/plugins/${encodeURIComponent(id)}/status`),
  install: (file: File): Promise<Msg<PluginStatus['manifest']>> => {
    const fd = new FormData();
    fd.append('file', file);
    return HttpUtil.post('/panel/api/plugins/install', fd);
  },
  start: (id: string) => HttpUtil.post(`/panel/api/plugins/${encodeURIComponent(id)}/start`, {}),
  stop: (id: string) => HttpUtil.post(`/panel/api/plugins/${encodeURIComponent(id)}/stop`, {}),
  restart: (id: string) =>
    HttpUtil.post(`/panel/api/plugins/${encodeURIComponent(id)}/restart`, {}),
  remove: (id: string) => HttpUtil.post(`/panel/api/plugins/${encodeURIComponent(id)}/delete`, {}),
  logs: (id: string, lines = 200): Promise<Msg<string[]>> =>
    HttpUtil.get(`/panel/api/plugins/${encodeURIComponent(id)}/logs?lines=${lines}`),
};
