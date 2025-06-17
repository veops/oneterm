import i18n from '@/lang'

export const STORAGE_CONFIG_TYPE = {
  LOCAL: 'local',
  MIN_IO: 'minio',
  S3: 's3',
  OSS: 'oss',
  COS: 'cos',
  AZURE: 'azure',
  OBS: 'obs',
  OOS: 'oos',
}

export const configTypeSelectOptions = [
  {
    value: STORAGE_CONFIG_TYPE.LOCAL,
    label: i18n.t('oneterm.storageConfig.local')
  },
  {
    value: STORAGE_CONFIG_TYPE.MIN_IO,
    label: 'MinIO'
  },
  {
    value: STORAGE_CONFIG_TYPE.S3,
    label: 'S3'
  },
  {
    value: STORAGE_CONFIG_TYPE.OSS,
    label: 'OSS'
  },
  {
    value: STORAGE_CONFIG_TYPE.COS,
    label: 'COS'
  },
  {
    value: STORAGE_CONFIG_TYPE.AZURE,
    label: 'Azure'
  },
  {
    value: STORAGE_CONFIG_TYPE.OBS,
    label: 'OBS'
  },
  {
    value: STORAGE_CONFIG_TYPE.OOS,
    label: 'OOS'
  }
]

export const COMMON_CONFIG_FORM = [
  {
    field: 'retention_days',
    label: 'oneterm.storageConfig.retentionDays',
    extra: '',
    required: true,
    component: 'a-input-number',
    componentProps: {
      min: 1,
      precision: 0,
    }
  },
  {
    field: 'archive_days',
    label: 'oneterm.storageConfig.archiveDays',
    extra: '',
    required: true,
    component: 'a-input-number',
    componentProps: {
      min: 1,
      precision: 0,
    }
  },
  {
    field: 'cleanup_enabled',
    label: 'oneterm.storageConfig.isCleanupEnabled',
    extra: '',
    component: 'a-switch',
    componentProps: {}
  },
  {
    field: 'archive_enabled',
    label: 'oneterm.storageConfig.isArchiveEnabled',
    extra: '',
    component: 'a-switch',
    componentProps: {}
  }
]

export const LOCAL_CONFIG_FORM = [
  {
    field: 'base_path',
    label: 'oneterm.storageConfig.storageBasePath',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  ...COMMON_CONFIG_FORM
]

export const MIN_IO_CONFIG_FORM = [
  {
    field: 'endpoint',
    label: 'oneterm.storageConfig.minIOEndpoint',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'access_key_id',
    label: 'oneterm.storageConfig.accessKeyID',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'secret_access_key',
    label: 'oneterm.storageConfig.secretAccessKey',
    extra: '',
    required: true,
    component: 'PasswordField',
    componentProps: {}
  },
  {
    field: 'bucket_name',
    label: 'oneterm.storageConfig.bucketName',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'use_ssl',
    label: 'oneterm.storageConfig.isUseSSL',
    extra: '',
    component: 'a-switch',
    componentProps: {}
  },
  ...COMMON_CONFIG_FORM
]

export const S3_CONFIG_FORM = [
  {
    field: 'region',
    label: 'oneterm.storageConfig.AWSRegion',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'access_key_id',
    label: 'oneterm.storageConfig.accessKeyID',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'secret_access_key',
    label: 'oneterm.storageConfig.secretAccessKey',
    extra: '',
    required: true,
    component: 'PasswordField',
    componentProps: {}
  },
  {
    field: 'bucket_name',
    label: 'oneterm.storageConfig.bucketName',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'endpoint',
    label: 'oneterm.storageConfig.endpoint',
    extra: '',
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'use_ssl',
    label: 'oneterm.storageConfig.isUseSSL',
    extra: '',
    component: 'a-switch',
    componentProps: {}
  },
  ...COMMON_CONFIG_FORM
]

export const OSS_CONFIG_FORM = [
  {
    field: 'endpoint',
    label: 'oneterm.storageConfig.OSSEndpoint',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'access_key_id',
    label: 'oneterm.storageConfig.accessKeyID',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'access_key_secret',
    label: 'oneterm.storageConfig.accessKeySecret',
    extra: '',
    required: true,
    component: 'PasswordField',
    componentProps: {}
  },
  {
    field: 'bucket_name',
    label: 'oneterm.storageConfig.bucketName',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  ...COMMON_CONFIG_FORM
]

export const COS_CONFIG_FORM = [
  {
    field: 'region',
    label: 'oneterm.storageConfig.COSRegion',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'secret_id',
    label: 'oneterm.storageConfig.secretID',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'secret_key',
    label: 'oneterm.storageConfig.secretKey',
    extra: '',
    required: true,
    component: 'PasswordField',
    componentProps: {}
  },
  {
    field: 'bucket_name',
    label: 'oneterm.storageConfig.bucketName',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'app_id',
    label: 'oneterm.storageConfig.appID',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  ...COMMON_CONFIG_FORM
]

export const AZURE_CONFIG_FORM = [
  {
    field: 'account_name',
    label: 'oneterm.storageConfig.accountName',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'account_key',
    label: 'oneterm.storageConfig.accountKey',
    extra: '',
    required: true,
    component: 'PasswordField',
    componentProps: {}
  },
  {
    field: 'container_name',
    label: 'oneterm.storageConfig.containerName',
    extra: '',
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'endpoint_suffix',
    label: 'oneterm.storageConfig.endpointSuffix',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  ...COMMON_CONFIG_FORM
]

export const OBS_CONFIG_FORM = [
  {
    field: 'endpoint',
    label: 'oneterm.storageConfig.OBSEndpoint',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'access_key_id',
    label: 'oneterm.storageConfig.accessKeyID',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'secret_access_key',
    label: 'oneterm.storageConfig.secretAccessKey',
    extra: '',
    required: true,
    component: 'PasswordField',
    componentProps: {}
  },
  {
    field: 'bucket_name',
    label: 'oneterm.storageConfig.bucketName',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  ...COMMON_CONFIG_FORM
]

export const OOS_CONFIG_FORM = [
  {
    field: 'endpoint',
    label: 'oneterm.storageConfig.OOSEndPoint',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'access_key_id',
    label: 'oneterm.storageConfig.accessKeyID',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'secret_access_key',
    label: 'oneterm.storageConfig.secretAccessKey',
    extra: '',
    required: true,
    component: 'PasswordField',
    componentProps: {}
  },
  {
    field: 'bucket_name',
    label: 'oneterm.storageConfig.bucketName',
    extra: '',
    required: true,
    component: 'a-input',
    componentProps: {}
  },
  {
    field: 'region',
    label: 'oneterm.storageConfig.region',
    extra: '',
    component: 'a-input',
    componentProps: {}
  },
  ...COMMON_CONFIG_FORM
]
