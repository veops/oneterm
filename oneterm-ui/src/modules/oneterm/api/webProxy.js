import { axios } from '@/utils/request'

export function startWebProxy(data, isShowMessage = true) {
  return axios({
    url: '/oneterm/v1/web_proxy/start',
    method: 'post',
    data,
    isShowMessage
  })
}
