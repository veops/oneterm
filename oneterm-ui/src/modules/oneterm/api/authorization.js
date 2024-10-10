import { axios } from '@/utils/request'

export function getAuth(params) {
    return axios({
        url: '/oneterm/v1/authorization',
        method: 'get',
        params
    })
}

export function postAuth(data) {
    return axios({
        url: '/oneterm/v1/authorization',
        method: 'post',
        data
    })
}
