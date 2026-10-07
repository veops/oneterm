export function passwordViewStateLabel(access, translate) {
  const state = access?.state
  const request = access?.request
  if (state === 'pending' && request && !request.ticket_id) {
    return translate('oneterm.passwordView.states.' + (request.sync_state === 'failed' ? 'submit_failed' : 'submitting'))
  }
  const known = ['required', 'not_required', 'configuration_required', 'approval_disabled', 'pending', 'approved', 'rejected', 'cancelled', 'expired', 'invalidated']
  return translate('oneterm.passwordView.states.' + (known.includes(state) ? state : 'unavailable'))
}
