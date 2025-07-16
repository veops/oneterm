import { axios } from '@/utils/request'

export function getAssetList(params) {
  return axios({
    url: '/oneterm/v1/asset',
    method: 'get',
    params
  })
}

export function postAsset(data) {
  return axios({
    url: '/oneterm/v1/asset',
    method: 'post',
    data
  })
}

export function putAssetById(id, data) {
  return axios({
    url: `/oneterm/v1/asset/${id}`,
    method: 'put',
    data
  })
}

export function deleteAssetById(id) {
  return axios({
    url: `/oneterm/v1/asset/${id}`,
    method: 'delete',
  })
}

export function getAssetPermissions(id, params) {
  return axios({
    url: `/oneterm/v1/asset/${id}/permissions`,
    method: 'get',
    params
  })
}
