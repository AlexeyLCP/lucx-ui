import axios from 'axios'

const LUCX_BASE = '/panel/api/lucx'

export async function postLucx(path, body) {
  const { data } = await axios.post(LUCX_BASE + path, body)
  return data
}

export async function getLucx(path) {
  const { data } = await axios.get(LUCX_BASE + path)
  return data
}
