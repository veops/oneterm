export function adoptionPayload(form) {
  if (!form.asset_id || !form.account_id || !form.platform || !['asset_local', 'shared'].includes(form.authority_kind)) {
    throw new Error('oneterm.pam.adoptionInvalid')
  }
  const reference = String(form.authority_ref || '').trim()
  if (form.authority_kind === 'shared' && !reference) throw new Error('oneterm.pam.authorityRequired')
  return {
    asset_id: Number(form.asset_id),
    account_id: Number(form.account_id),
    authority_kind: form.authority_kind,
    authority_ref: form.authority_kind === 'shared' ? reference : '',
    platform: form.platform,
    native_qualifier: String(form.native_qualifier || '').trim()
  }
}

export function policyPayload(form, sourceText) {
  return {
    revision: form.revision,
    allow_requests: Boolean(form.allow_requests),
    approval_provider: form.approval_provider,
    required_approvals: Number(form.required_approvals),
    max_duration_minutes: Number(form.max_duration_minutes),
    request_expiry_minutes: Number(form.request_expiry_minutes),
    require_approval_actions: (form.require_approval_actions || []).filter(action => action === 'connect'),
    source_cidrs: [...new Set(String(sourceText || '').split(/[\s,]+/).filter(Boolean))],
    allow_applications: Boolean(form.allow_applications),
    connection_permissions: {
      connect: true,
      file_upload: Boolean(form.connection_permissions.file_upload),
      file_download: Boolean(form.connection_permissions.file_download),
      copy: Boolean(form.connection_permissions.copy),
      paste: Boolean(form.connection_permissions.paste),
      share: false
    },
    max_sessions: Number(form.max_sessions),
    max_session_minutes: Number(form.max_session_minutes),
    itsm_template_id: form.itsm_template_id || 0,
    itsm_template_revision: form.itsm_template_revision || ''
  }
}
