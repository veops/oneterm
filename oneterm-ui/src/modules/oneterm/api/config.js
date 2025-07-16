import { axios } from '@/utils/request'

export function getConfig(params) {
  return axios({
    url: '/oneterm/v1/config',
    method: 'get',
    params
  })
}

export function postConfig(data) {
  return axios({
    url: '/oneterm/v1/config',
    method: 'post',
    data
  })
}
