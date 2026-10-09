import { axios } from '@/utils/request'

export function getAccountList(params) {
  return axios({
    url: '/oneterm/v1/account',
    method: 'get',
    params
  })
}

export function postAccount(data) {
  return axios({
    url: '/oneterm/v1/account',
    method: 'post',
    data
  })
}

export function putAccountById(id, data) {
  return axios({
    url: `/oneterm/v1/account/${id}`,
    method: 'put',
    data
  })
}

export function deleteAccountById(id) {
  return axios({
    url: `/oneterm/v1/account/${id}`,
    method: 'delete',
  })
}

export function getAccountByCredentials(id) {
  return axios({
    url: `/oneterm/v1/account/${id}/credentials2`,
    method: 'get',
  })
}
