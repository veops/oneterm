import { axios } from '@/utils/request'

export function closeConnect(session_id) {
    return axios({
        url: `/oneterm/v1/connect/close/${session_id}`,
        method: 'post',
    })
}

export function postConnectIsRight(asset_id, account_id, protocol) {
    return axios({
        url: `/oneterm/v1/connect/${asset_id}/${account_id}/${protocol}`,
        method: 'post',
    })
}
