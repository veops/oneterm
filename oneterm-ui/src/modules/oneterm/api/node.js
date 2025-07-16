import { axios } from '@/utils/request'

export function getNodeList(params) {
  return axios({
    url: '/oneterm/v1/node',
    method: 'get',
    params
  })
}

export function getNodeById(id) {
  return axios({
    url: `/oneterm/v1/node?id=${id}`,
    method: 'get',
  })
}

export function postNode(data) {
  return axios({
    url: '/oneterm/v1/node',
    method: 'post',
    data
  })
}

export function putNodeById(id, data) {
  return axios({
    url: `/oneterm/v1/node/${id}`,
    method: 'put',
    data
  })
}

export function deleteNodeById(id) {
  return axios({
    url: `/oneterm/v1/node/${id}`,
    method: 'delete',
  })
}
