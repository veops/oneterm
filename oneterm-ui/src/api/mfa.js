import { axios } from '@/utils/request'

export function MFARevoke() {
  return axios({
    url: '/common-setting/v1/mfa/revoke',
    method: 'post',
    timeout: 1500,
    isShowMessage: false
  })
}

export function MFAChallenge(data) {
  return axios({
    url: '/common-setting/v1/mfa/challenge',
    method: 'post',
    data
  })
}

export function MFAVerify(data) {
  return axios({
    url: '/common-setting/v1/mfa/verify',
    method: 'post',
    data
  })
}
