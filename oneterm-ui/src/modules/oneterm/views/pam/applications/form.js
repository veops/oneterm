export function applicationPayload(form, options) {
  const targets = Object.keys(form.targets || {}).reduce((result, kind) => {
    result[kind] = [...new Set((form.targets[kind] || []).map(Number))]
    return result
  }, {})
  return {
    name: form.name.trim(),
    acl_uid: Number(form.acl_uid),
    purpose: form.purpose.trim(),
    enabled: Boolean(form.enabled),
    actions: [...form.actions],
    targets,
    source_cidrs: options.restrictSource
      ? [...new Set(options.sourceText.split(/[\n,]+/).map((value) => value.trim()).filter(Boolean))]
      : [],
    expires_at: options.permanent ? null : options.expiresAt.toISOString(),
    revision: Number(form.revision || 0)
  }
}

export function applicationState(application, now = Date.now()) {
  if (!application.enabled) return 'disabled'
  if (application.expires_at && new Date(application.expires_at).getTime() <= now) return 'expired'
  return 'enabled'
}
