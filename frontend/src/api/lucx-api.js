import axios from 'axios'

const LUCX_BASE = '/panel/api/lucx'

// Safe JSON-aware POST — bulletproof against non-JSON responses
export async function postLucx(path, body) {
  try {
    const resp = await axios.post(LUCX_BASE + path, body, {
      headers: { 'Content-Type': 'application/json' },
      transformResponse: [(data) => {
        // Never let axios throw on JSON parse — handle gracefully
        if (typeof data === 'string') {
          try { return JSON.parse(data); } catch (_) { /* pass */ }
        }
        return data;
      }],
    })
    const data = resp.data
    if (typeof data !== 'object' || data === null) {
      return { success: false, msg: typeof data === 'string' ? data.slice(0, 200) : 'Non-JSON response' }
    }
    return data
  } catch (e) {
    // Network error, timeout, or non-2xx with non-JSON body
    const respData = e?.response?.data
    if (typeof respData === 'object' && respData !== null) {
      return respData
    }
    const status = e?.response?.status || 0
    const msg = (typeof respData === 'string' && respData)
      ? respData.slice(0, 200)
      : (e?.message || 'Network error')
    return { success: false, msg: msg || ('HTTP ' + status) }
  }
}

export async function getLucx(path) {
  try {
    const resp = await axios.get(LUCX_BASE + path, {
      transformResponse: [(data) => {
        if (typeof data === 'string') {
          try { return JSON.parse(data); } catch (_) {}
        }
        return data;
      }],
    })
    const data = resp.data
    if (typeof data !== 'object' || data === null) {
      return { success: false, msg: 'Non-JSON response' }
    }
    return data
  } catch (e) {
    const respData = e?.response?.data
    if (typeof respData === 'object' && respData !== null) return respData
    return { success: false, msg: e?.message || 'Network error' }
  }
}
