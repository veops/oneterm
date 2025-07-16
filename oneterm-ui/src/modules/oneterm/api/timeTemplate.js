import { axios } from '@/utils/request'

export function getTimeTemplateList(params) {
  return axios({
    url: '/oneterm/v1/time_template',
    method: 'get',
    params
  })
}

export function postTimeTemplate(data) {
  return axios({
    url: '/oneterm/v1/time_template',
    method: 'post',
    data
  })
}

export function putTimeTemplateById(id, data) {
  return axios({
    url: `/oneterm/v1/time_template/${id}`,
    method: 'put',
    data
  })
}

export function deleteTimeTemplateById(id) {
  return axios({
    url: `/oneterm/v1/time_template/${id}`,
    method: 'delete',
  })
}
