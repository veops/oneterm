import { axios } from '@/utils/request'

export function getQuickCommand(params) {
  return axios({
    url: '/oneterm/v1/quick_command',
    method: 'get',
    params
  })
}

export function postQuickCommand(data) {
  return axios({
    url: '/oneterm/v1/quick_command',
    method: 'post',
    data
  })
}

export function putQuickCommandById(id, data) {
  return axios({
    url: `/oneterm/v1/quick_command/${id}`,
    method: 'put',
    data
  })
}

export function deleteQuickCommandById(id) {
  return axios({
    url: `/oneterm/v1/quick_command/${id}`,
    method: 'delete',
  })
}
