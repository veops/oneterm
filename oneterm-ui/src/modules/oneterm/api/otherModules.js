import { axios } from '@/utils/request'

export function getCITypeGroups(params) {
  return axios({
    url: `/v0.1/ci_types/groups`,
    method: 'GET',
    params: params
  })
}

export function getCITypeAttributesById(CITypeId, parameter) {
  return axios({
    url: `/v0.1/ci_types/${CITypeId}/attributes`,
    method: 'get',
    params: parameter
  })
}
