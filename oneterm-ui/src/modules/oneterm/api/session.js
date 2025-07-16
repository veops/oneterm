import { axios } from '@/utils/request'

export function getSessionList(params) {
  return axios({
    url: '/oneterm/v1/session',
    method: 'get',
    params
  })
}

export function getSessionCmdList(session_id, params) {
  return axios({
    url: `/oneterm/v1/session/${session_id}/cmd`,
    method: 'get',
    params
  })
}
