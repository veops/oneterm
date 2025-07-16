import { axios } from '@/utils/request'

export function getCountStat(params) {
  return axios({
    url: '/oneterm/v1/stat/count',
    method: 'get',
    params
  })
}

export function getAssetTypeStat(params) {
  return axios({
    url: '/oneterm/v1/stat/assettype',
    method: 'get',
    params
  })
}

export function getAssetStat(params) {
  return axios({
    url: '/oneterm/v1/stat/asset',
    method: 'get',
    params
  })
}

export function getAccountStat(params) {
  return axios({
    url: '/oneterm/v1/stat/account',
    method: 'get',
    params
  })
}

export function getOfUserStat(params) {
  return axios({
    url: '/oneterm/v1/stat/count/ofuser',
    method: 'get',
    params
  })
}

export function getRankOfUserStat(params) {
  return axios({
    url: '/oneterm/v1/stat/rank/ofuser',
    method: 'get',
    params
  })
}
