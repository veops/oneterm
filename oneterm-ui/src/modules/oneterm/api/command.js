import { axios } from '@/utils/request'

export function getCommandList(params) {
  return axios({
    url: '/oneterm/v1/command',
    method: 'get',
    params
  })
}

export function postCommand(data) {
  return axios({
    url: '/oneterm/v1/command',
    method: 'post',
    data
  })
}

export function putCommandById(id, data) {
  return axios({
    url: `/oneterm/v1/command/${id}`,
    method: 'put',
    data
  })
}

export function deleteCommandById(id) {
  return axios({
    url: `/oneterm/v1/command/${id}`,
    method: 'delete',
  })
}
