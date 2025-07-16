import { axios } from '@/utils/request'

export function getGatewayList(params) {
  return axios({
    url: '/oneterm/v1/gateway',
    method: 'get',
    params
  })
}

export function postGateway(data) {
  return axios({
    url: '/oneterm/v1/gateway',
    method: 'post',
    data
  })
}

export function putGatewayById(id, data) {
  return axios({
    url: `/oneterm/v1/gateway/${id}`,
    method: 'put',
    data
  })
}

export function deleteGatewayById(id) {
  return axios({
    url: `/oneterm/v1/gateway/${id}`,
    method: 'delete',
  })
}
