// All active experiments for one learner must be reachable from one sandbox.
export function experimentNode(
  attacker: string | null | undefined,
  targets: string[]
): string | null {
  const nodes = new Set([...targets, ...(attacker ? [attacker] : [])])
  if (nodes.size > 1) throw new Error('Conflicting experiment nodes')
  return [...nodes][0] || null
}

export function targetReplyState(
  previous: string,
  action: string,
  reply: { error?: string; instance?: { state: string } }
): string {
  // A missing ledger or a stop tombstone does not confirm Docker removal.
  if (reply.error && !reply.instance) return action === 'stop' ? 'stopping' : 'unconfirmed'
  return reply.instance?.state || (action === 'stop' ? 'stopping' : previous)
}
