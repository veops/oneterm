import { axios } from '@/utils/request'

const prefix = '/oneterm/v1/pam'

export const getPasswordViewStatus = (kind, id) => axios({ url: `${prefix}/password-view/${kind}/${id}`, method: 'get', isShowMessage: false })
export const getPasswordViewAccounts = (params) => axios({ url: `${prefix}/password-view/accounts`, method: 'get', params })
export const readPasswordView = (kind, id, token) => axios({ url: `${prefix}/password-view/${kind}/${id}/read`, method: 'post', headers: { 'X-MFA-Token': token }, isShowMessage: false })
export const getPasswordViewAudit = (params) => axios({ url: `${prefix}/password-view/audit`, method: 'get', params })

export const getPAMCapabilities = () => axios({ url: `${prefix}/capabilities`, method: 'get' })
export const getPAMAccount = (id) => axios({ url: `${prefix}/accounts/${id}`, method: 'get' })
export const getPAMAccountBindings = (id, params) => axios({ url: `${prefix}/accounts/${id}/bindings`, method: 'get', params })
export const getPAMPasswordExecutors = (id, params) => axios({ url: `${prefix}/accounts/${id}/password/executors`, method: 'get', params })
export const getPAMPasswordExecutions = (id, params) => axios({ url: `${prefix}/accounts/${id}/password/executions`, method: 'get', params })
export const savePAMPasswordConfig = (id, bindingId, data, token) => axios({
  url: `${prefix}/accounts/${id}/bindings/${bindingId}/password`,
  method: 'put',
  data,
  headers: { 'X-MFA-Token': token },
  isShowMessage: false
})
export const verifyPAMPassword = (id, data) => axios({
  url: `${prefix}/accounts/${id}/password/verify`, method: 'post', data, timeout: 45000, isShowMessage: false
})
export const startPAMPasswordChange = (id, data, token) => axios({
  url: `${prefix}/accounts/${id}/password/executions`,
  method: 'post',
  data,
  timeout: 95000,
  headers: { 'X-MFA-Token': token },
  isShowMessage: false
})
export const continuePAMPassword = (id, executionId, operation, token, data) => axios({
  url: `${prefix}/accounts/${id}/password/executions/${executionId}/${operation}`,
  method: 'post',
  data,
  timeout: 95000,
  headers: { 'X-MFA-Token': token },
  isShowMessage: false
})
export const getPAMAdoptionAssets = (params) => axios({ url: `${prefix}/adoption/assets`, method: 'get', params })
export const getPAMPolicy = (id) => axios({ url: `${prefix}/policies/account/${id}`, method: 'get' })
export const adoptPAMAccount = (data, token) => axios({ url: `${prefix}/accounts/adopt`, method: 'post', data, headers: { 'X-MFA-Token': token }, isShowMessage: false })
export const setPAMAccountEnabled = (id, revision, enabled, token) => axios({
  url: `${prefix}/accounts/${id}/${enabled ? 'enable' : 'disable'}`, method: 'post', data: { revision }, headers: { 'X-MFA-Token': token }, isShowMessage: false
})
export const removeAccountManagement = (id, revision, token) => axios({
  url: `${prefix}/accounts/${id}/management`, method: 'delete', data: { revision }, headers: { 'X-MFA-Token': token }, isShowMessage: false
})
export const attachPAMAccount = (id, data, token) => axios({ url: `${prefix}/accounts/${id}/bindings`, method: 'post', data, headers: { 'X-MFA-Token': token }, isShowMessage: false })
export const reviewPAMBinding = (id, data, token) => axios({ url: `${prefix}/accounts/${id}/bindings`, method: 'put', data, headers: { 'X-MFA-Token': token }, isShowMessage: false })
export const savePAMPolicy = (id, data, token) => axios({ url: `${prefix}/policies/account/${id}`, method: 'put', data, headers: { 'X-MFA-Token': token }, isShowMessage: false })
export const cancelPAMRequest = (id, data, token) => axios({
  url: `${prefix}/requests/${id}/cancel`, method: 'post', data, headers: token ? { 'X-MFA-Token': token } : {}, isShowMessage: false
})
export const retrievePAMCredential = (kind, id, token) => axios({
  url: `${prefix}/credentials/${kind}/${id}`, method: 'post', headers: { 'X-MFA-Token': token }, isShowMessage: false
})
export const getPAMApplications = (params) => axios({ url: `${prefix}/applications`, method: 'get', params })
export const getPAMTargets = (params) => axios({ url: `${prefix}/targets`, method: 'get', params })
export const getPAMAudit = (params) => axios({ url: `${prefix}/audit`, method: 'get', params })
export const getPAMSubjects = (params) => axios({
  url: '/v1/acl/users', method: 'get', params: { ...params, metadata_only: true }
})
export const createPAMApplication = (data, token) => axios({
  url: `${prefix}/applications`, method: 'post', data, headers: { 'X-MFA-Token': token }, isShowMessage: false
})
export const updatePAMApplication = (id, data, token) => axios({
  url: `${prefix}/applications/${id}`, method: 'put', data, headers: { 'X-MFA-Token': token }, isShowMessage: false
})
export const disablePAMApplication = (id, revision, token) => axios({
  url: `${prefix}/applications/${id}/disable`,
  method: 'post',
  data: { revision },
  headers: { 'X-MFA-Token': token },
  isShowMessage: false
})
