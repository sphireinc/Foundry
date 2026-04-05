export const createSessionViews = ({
  state,
  panel,
  escapeHTML,
  formatDateTime,
  renderTableControls,
  sortItems,
  paginateItems,
}) => {
  const renderSessions = () => {
    const normalizedFilter = String(state.sessionFilters?.username || '').trim().toLowerCase();
    const visibleSessions = (state.userSessions || []).filter((session) => {
      if (!normalizedFilter) return true;
      return String(session.username || '').trim().toLowerCase().includes(normalizedFilter);
    });
    const sortedSessions = sortItems(visibleSessions, 'sessions', (session, field) => {
      switch (field) {
        case 'username':
          return session.username;
        case 'issued_at':
          return session.issued_at;
        case 'expires_at':
          return session.expires_at;
        case 'last_seen':
        default:
          return session.last_seen;
      }
    });
    const pagedSessions = paginateItems(sortedSessions, 'sessions');
    const now = Date.now();
    const sessionAgeHours = (session) => {
      const issued = Date.parse(String(session.issued_at || ''));
      if (!Number.isFinite(issued)) return 0;
      return Math.max(0, (now - issued) / (1000 * 60 * 60));
    };
    const sessionIdleMinutes = (session) => {
      const lastSeen = Date.parse(String(session.last_seen || ''));
      if (!Number.isFinite(lastSeen)) return 0;
      return Math.max(0, (now - lastSeen) / (1000 * 60));
    };
    const usernameCounts = new Map();
    const addressSets = new Map();
    for (const session of visibleSessions) {
      const username = String(session.username || '').trim().toLowerCase();
      if (!username) continue;
      usernameCounts.set(username, (usernameCounts.get(username) || 0) + 1);
      if (!addressSets.has(username)) addressSets.set(username, new Set());
      if (String(session.remote_addr || '').trim()) {
        addressSets.get(username).add(String(session.remote_addr || '').trim());
      }
    }
    const longLivedCount = visibleSessions.filter((session) => sessionAgeHours(session) >= 12).length;
    const idleCount = visibleSessions.filter((session) => sessionIdleMinutes(session) >= 30).length;
    const sharedUsers = Array.from(usernameCounts.values()).filter((count) => count > 1).length;
    const spreadUsers = Array.from(addressSets.values()).filter((set) => set.size > 1).length;
    const clientLabel = (session) => {
      const agent = String(session.user_agent || '').trim();
      if (!agent) return session.current ? 'Current Browser Session' : 'Unknown Client';
      const lower = agent.toLowerCase();
      if (lower.includes('iphone') || lower.includes('ios')) return `iPhone / iOS${session.current ? ' (Current)' : ''}`;
      if (lower.includes('ipad')) return `iPad / iPadOS${session.current ? ' (Current)' : ''}`;
      if (lower.includes('android')) return `Android Device${session.current ? ' (Current)' : ''}`;
      if (lower.includes('mac os') || lower.includes('macintosh')) return `Mac Browser${session.current ? ' (Current)' : ''}`;
      if (lower.includes('windows')) return `Windows Browser${session.current ? ' (Current)' : ''}`;
      if (lower.includes('linux')) return `Linux Browser${session.current ? ' (Current)' : ''}`;
      if (lower.includes('curl')) return `CLI Client${session.current ? ' (Current)' : ''}`;
      return session.current ? `${agent} (Current)` : agent;
    };
    const addressFingerprint = (session) => {
      const raw = String(session.remote_addr || '').trim();
      if (!raw) return '-';
      const hash = Array.from(raw).reduce((acc, char) => (acc * 33 + char.charCodeAt(0)) >>> 0, 5381);
      if (raw.includes('.')) {
        const parts = raw.split('.');
        const suffix = parts.slice(-2).join('.');
        return `x.x.${suffix} #${hash.toString(16).slice(-6)}`;
      }
      if (raw.includes(':')) {
        const parts = raw.split(':').filter(Boolean);
        const suffix = parts.slice(-2).join(':') || raw.slice(-8);
        return `x:x:${suffix} #${hash.toString(16).slice(-6)}`;
      }
      const half = raw.slice(Math.max(0, Math.floor(raw.length / 2)));
      return `...${half} #${hash.toString(16).slice(-6)}`;
    };
    const sessionFlags = (session) => {
      const username = String(session.username || '').trim().toLowerCase();
      const flags = [];
      if (session.current) flags.push('Current');
      if (sessionAgeHours(session) >= 12) flags.push('Long-Lived');
      if (sessionIdleMinutes(session) >= 30) flags.push('Idle');
      if ((usernameCounts.get(username) || 0) > 1) flags.push('Concurrent');
      if ((addressSets.get(username)?.size || 0) > 1) flags.push('Address Spread');
      return flags;
    };
    const rows = pagedSessions.items.map(
      (session) => `
      <div class="table-row table-row-actions">
        <span>
          <label class="checkbox inline-checkbox">
            <input type="checkbox" data-select-session="${escapeHTML(session.id || '')}" ${state.selectedSessions.includes(session.id) ? 'checked' : ''}>
            <strong>${escapeHTML(clientLabel(session))}</strong>
          </label>
          <div class="muted">${escapeHTML(session.user_agent || 'Unknown client')}</div>
          <div class="muted mono">${escapeHTML(session.id || '')}</div>
          <div class="toolbar">
            ${sessionFlags(session)
              .map((flag) => `<span class="contract-badge ${flag === 'Current' ? 'ok' : 'warn'}">${escapeHTML(flag)}</span>`)
              .join('')}
          </div>
        </span>
        <span>
          <strong>${escapeHTML(session.username || '')}</strong>
          <div class="muted">${escapeHTML(session.role || '')}</div>
        </span>
        <span>
          <div>${escapeHTML(addressFingerprint(session))}</div>
          <div class="muted">${session.mfa_complete ? 'MFA complete' : 'Password only'}</div>
        </span>
        <span>
          <div>${escapeHTML(formatDateTime(session.last_seen) || session.last_seen || '-')}</div>
          <div class="muted">Issued ${escapeHTML(formatDateTime(session.issued_at) || session.issued_at || '-')}</div>
          <div class="muted">Expires ${escapeHTML(formatDateTime(session.expires_at) || session.expires_at || '-')}</div>
        </span>
        <span class="row-actions">
          <button type="button" class="ghost small" data-session-user="${escapeHTML(session.username || '')}">Open User</button>
          <button type="button" class="ghost small danger" data-revoke-session-id="${escapeHTML(session.id || '')}" ${session.current ? 'data-current-session="true"' : ''}>Revoke</button>
        </span>
      </div>`
    );
    return `
      <div class="layout-grid">
        <div class="stack">
          ${panel(
            'Sessions',
            `
            <div class="panel-pad">
              <div class="cards">
                <article class="card"><span class="card-label">Concurrent Users</span><strong>${escapeHTML(String(sharedUsers))}</strong><span class="card-copy">Users with multiple active sessions.</span></article>
                <article class="card"><span class="card-label">Address Spread</span><strong>${escapeHTML(String(spreadUsers))}</strong><span class="card-copy">Users with sessions from multiple addresses.</span></article>
                <article class="card"><span class="card-label">Long-Lived</span><strong>${escapeHTML(String(longLivedCount))}</strong><span class="card-copy">Sessions older than 12 hours.</span></article>
                <article class="card"><span class="card-label">Idle</span><strong>${escapeHTML(String(idleCount))}</strong><span class="card-copy">Sessions idle for 30+ minutes.</span></article>
              </div>
            </div>
            <div class="panel-pad stack">
              <form id="session-filter-form" class="toolbar">
                <label>Username
                  <input id="session-filter-username" type="text" value="${escapeHTML(state.sessionFilters?.username || '')}" placeholder="Filter by username">
                </label>
                <button type="submit" class="ghost small">Apply Filter</button>
                <button type="button" class="ghost small" id="session-filter-clear">Clear</button>
                <button type="button" class="ghost small" id="session-select-all">${pagedSessions.items.length && pagedSessions.items.every((session) => state.selectedSessions.includes(session.id)) ? 'Deselect All' : 'Select All'}</button>
                <button type="button" class="ghost small danger" id="session-revoke-selected" ${state.selectedSessions.length ? '' : 'disabled'}>Revoke Selected</button>
                <button type="button" class="small danger" id="session-revoke-all">Emergency Revoke All</button>
              </form>
            </div>
            ${renderTableControls(state, 'sessions', visibleSessions.length, pagedSessions.totalPages)}
            <div class="table table-five"><div class="table-head"><span>Client</span><span>User</span><span>Network</span><span>Activity</span><span>Actions</span></div>${rows.length ? rows.join('') : '<div class="panel-pad empty-state">No active sessions found.</div>'}</div>`,
            `${visibleSessions.length} active sessions`
          )}
        </div>
        <div class="stack">
          ${panel(
            'Session Notes',
            `<div class="panel-pad stack">
              <div class="note">Sessions are stored with a hashed bearer token, a non-secret session id, coarse remote address, and truncated user-agent for operational review.</div>
              <div class="note">Revoking the current session will sign that browser out on its next authenticated action.</div>
              <div class="note">Address display is intentionally masked and fingerprinted so operators can distinguish sessions without exposing the full stored address in the UI.</div>
              <div class="note">Signals shown here are heuristic only: concurrent sessions, address spread, long-lived sessions, and idle sessions help operators spot suspicious patterns quickly.</div>
              <div class="note">Per-user session controls remain available from the Users screen.</div>
            </div>`,
            'Operational visibility without storing raw session tokens'
          )}
        </div>
      </div>`;
  };

  return {
    renderSessions,
  };
};
