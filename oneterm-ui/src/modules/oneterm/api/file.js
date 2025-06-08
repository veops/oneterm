import { axios } from '@/utils/request'

export function getFileList(assetId, accountId, params = {}) {
  return axios({
    url: `/oneterm/v1/file/ls/${assetId}/${accountId}`,
    method: 'get',
    params
  })
}

export function getFileListBySessionId(sessionId, params = {}) {
  return axios({
    url: `/oneterm/v1/file/session/${sessionId}/ls`,
    method: 'get',
    params
  })
}

export function getFileHistory(params) {
  return axios({
    url: `/oneterm/v1/file/history`,
    method: 'get',
    params
  })
}

export function getFileTransferProgressById(transferId, params) {
  return axios({
    url: `/oneterm/v1/file/transfer/progress/id/${transferId}`,
    method: 'get',
    params
  })
}
