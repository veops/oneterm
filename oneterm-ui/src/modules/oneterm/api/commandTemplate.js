import { axios } from '@/utils/request'

export function getCommandTemplateList(params) {
  return axios({
    url: '/oneterm/v1/command_template',
    method: 'get',
    params
  })
}

export function postCommandTemplate(data) {
  return axios({
    url: '/oneterm/v1/command_template',
    method: 'post',
    data
  })
}

export function putCommandTemplateById(id, data) {
  return axios({
    url: `/oneterm/v1/command_template/${id}`,
    method: 'put',
    data
  })
}

export function deleteCommandTemplateById(id) {
  return axios({
    url: `/oneterm/v1/command_template/${id}`,
    method: 'delete',
  })
}
