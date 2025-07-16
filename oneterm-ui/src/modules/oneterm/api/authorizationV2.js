import { axios } from '@/utils/request'

export function getAuthList(params) {
  return axios({
    url: '/oneterm/v1/authorization_v2',
    method: 'get',
    params
  })
}

export function postAuth(data) {
  return axios({
    url: '/oneterm/v1/authorization_v2',
    method: 'post',
    data
  })
}

export function putAuthById(id, data) {
  return axios({
    url: `/oneterm/v1/authorization_v2/${id}`,
    method: 'put',
    data
  })
}

export function deleteAuthById(id) {
  return axios({
    url: `/oneterm/v1/authorization_v2/${id}`,
    method: 'delete',
  })
}
