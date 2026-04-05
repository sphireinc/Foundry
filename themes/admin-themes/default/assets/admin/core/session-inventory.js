export const createSessionInventory = ({
  admin,
  state,
  render,
  capabilityInfoHas,
}) => {
  const loadSessionInventory = async ({ rerender = true, force = false } = {}) => {
    if (!capabilityInfoHas('users.manage')) {
      state.userSessions = [];
      state.userSessionsLoaded = true;
      return;
    }
    if (state.userSessionsLoaded && !force) {
      return;
    }
    try {
      const sessions = await admin.session.list();
      state.userSessions = Array.isArray(sessions) ? sessions : [];
      state.selectedSessions = state.selectedSessions.filter((sessionID) =>
        state.userSessions.some((session) => session.id === sessionID)
      );
      state.userSessionsLoaded = true;
    } catch (error) {
      state.userSessions = [];
      state.selectedSessions = [];
      const message = error?.message || String(error);
      const unavailable = String(message).includes('404');
      state.userSessionsLoaded = unavailable;
      if (rerender && !unavailable) {
        state.error = error.message || String(error);
      }
    } finally {
      if (rerender) {
        render();
      }
    }
  };

  return {
    loadSessionInventory,
  };
};
