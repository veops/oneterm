import { axios } from '@/utils/request'

export function getRDPFileList(sessionId, params = {}) {
  return axios({
    url: `/oneterm/v1/rdp/sessions/${sessionId}/files`,
    method: 'get',
    params
  })
}
