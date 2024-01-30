import { axios } from '@/utils/request'

const urlPrefix = '/v1/acl'

export function searchResource(params) {
  return axios({
    url: urlPrefix + `/resources`,
    method: 'GET',
    params: params
  })
}

export function addResource(params) {
  return axios({
    url: urlPrefix + '/resources',
    method: 'POST',
    data: params
  })
}

export function updateResourceById(id, params) {
  return axios({
    url: urlPrefix + `/resources/${id}`,
    method: 'PUT',
    data: params
  })
}

export function deleteResourceById(id, params) {
  return axios({
    url: urlPrefix + `/resources/${id}`,
    method: 'DELETE',
    params
  })
}

export function searchResourceType(params) {
  return axios({
    url: urlPrefix + `/resource_types`,
    method: 'GET',
    params: params
  })
}

export function addResourceType(params) {
  return axios({
    url: urlPrefix + '/resource_types',
    method: 'POST',
    data: params
  })
}

export function updateResourceTypeById(id, params) {
  return axios({
    url: urlPrefix + `/resource_types/${id}`,
    method: 'PUT',
    data: params
  })
}

export function deleteResourceTypeById(id) {
  return axios({
    url: urlPrefix + `/resource_types/${id}`,
    method: 'DELETE'
  })
}

// add resource group
export function getResourceGroups(params) {
  return axios({
    url: `${urlPrefix}/resource_groups`,
    method: 'GET',
    params: params
  })
}

export function addResourceGroup(data) {
  return axios({
    url: `${urlPrefix}/resource_groups`,
    method: 'POST',
    data: data
  })
}

export function updateResourceGroup(_id, data) {
  return axios({
    url: `${urlPrefix}/resource_groups/${_id}`,
    method: 'PUT',
    data: data
  })
}

export function deleteResourceGroup(_id) {
  return axios({
    url: `${urlPrefix}/resource_groups/${_id}`,
    method: 'DELETE'
  })
}

export function getResourceGroupItems(_id) {
  return axios({
    url: `${urlPrefix}/resource_groups/${_id}/items`,
    method: 'GET'
  })
}
