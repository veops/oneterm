import { axios } from '@/utils/request'

export function getStorageConfigs(params) {
  return axios({
    url: `/oneterm/v1/storage/configs`,
    method: 'get',
    params
  })
}

export function postStorageConfigs(data) {
  return axios({
    url: `/oneterm/v1/storage/configs`,
    method: 'post',
    data
  })
}

export function getStorageConfigsById(id) {
  return axios({
    url: `/oneterm/v1/storage/configs/${id}`,
    method: 'get'
  })
}

export function putStorageConfigs(id, data) {
  return axios({
    url: `/oneterm/v1/storage/configs/${id}`,
    method: 'put',
    data
  })
}

export function deleteStorageConfigs(id) {
  return axios({
    url: `/oneterm/v1/storage/configs/${id}`,
    method: 'delete',
  })
}

export function setPrimaryStorageConfig(id) {
  return axios({
    url: `/oneterm/v1/storage/configs/${id}/set-primary`,
    method: 'put',
  })
}

export function toggleEnabled(id) {
  return axios({
    url: `/oneterm/v1/storage/configs/${id}/toggle`,
    method: 'put',
  })
}

export function getStorageHealth() {
  return axios({
    url: `/oneterm/v1/storage/health`,
    method: 'get'
  })
}

export function testConnection(data) {
  return axios({
    url: `/oneterm/v1/storage/test-connection`,
    method: 'post',
    data
  })
}
