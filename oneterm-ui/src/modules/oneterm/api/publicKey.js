import { axios } from '@/utils/request'

export function getPublicKeyList(params) {
  return axios({
    url: `/oneterm/v1/public_key`,
    method: 'GET',
    params: params
  })
}

export function addPublicKey(data) {
  return axios({
    url: `/oneterm/v1/public_key`,
    method: 'POST',
    data: data
  })
}

export function putPublicKeyById(id, data) {
  return axios({
    url: `/oneterm/v1/public_key/${id}`,
    method: 'put',
    data
  })
}

export function deletePublicKeyById(id) {
  return axios({
    url: `/oneterm/v1/public_key/${id}`,
    method: 'delete',
  })
}
