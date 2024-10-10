import { axios } from '@/utils/request'

export function closeConnect(session_id) {
    return axios({
        url: `/oneterm/v1/connect/close/${session_id}`,
        method: 'post',
    })
}

export function postConnectIsRight(asset_id, account_id, protocol, query = null) {
    let url = `/oneterm/v1/connect/${asset_id}/${account_id}/${protocol}`
    if (query) {
        url = `${url}?${query}`
    }
    return axios({
        url,
        method: 'post',
    })
}

export function postShareLink(data) {
  return axios({
    url: `/oneterm/v1/share`,
    method: 'post',
    data
  })
}

export function getShareLink(params) {
  return axios({
    url: `/oneterm/v1/share`,
    method: 'get',
    params
  })
}
