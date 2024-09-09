import { axios } from '@/utils/request'

export function getSessionReplayData(session_id) {
  return axios({
    url: `/oneterm/v1/session/replay/${session_id}`,
    method: 'get'
  })
}
