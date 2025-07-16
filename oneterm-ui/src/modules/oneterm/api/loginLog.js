import { axios } from '@/utils/request'

export function getLoginLogList(params) {
  return axios({
    url: `/v1/acl/audit_log/login`,
    method: 'GET',
    params: params
  })
}
