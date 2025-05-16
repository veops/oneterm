import { axios } from '@/utils/request'

export function getPreference() {
  return axios({
    url: '/oneterm/v1/preference',
    method: 'get'
  })
}

export function putPreference(data) {
  return axios({
    url: '/oneterm/v1/preference',
    method: 'put',
    data
  })
}
