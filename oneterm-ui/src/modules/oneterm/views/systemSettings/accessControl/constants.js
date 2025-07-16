export const PERMISSION_TYPE = {
  CONNECT: 'connect',
  SHARE: 'share',
  FILE_UPLOAD: 'file_upload',
  FILE_DOWNLOAD: 'file_download',
  COPY: 'copy',
  PASTE: 'paste'
}

export const PERMISSION_TYPE_NAME = {
  [PERMISSION_TYPE.CONNECT]: 'oneterm.accessControl.connect',
  [PERMISSION_TYPE.FILE_UPLOAD]: 'oneterm.accessControl.upload',
  [PERMISSION_TYPE.FILE_DOWNLOAD]: 'oneterm.accessControl.download',
  [PERMISSION_TYPE.COPY]: 'oneterm.accessControl.copy',
  [PERMISSION_TYPE.PASTE]: 'oneterm.accessControl.paste',
  [PERMISSION_TYPE.SHARE]: 'oneterm.accessControl.share',
}
