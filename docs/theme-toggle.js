(() => {
  const cookieName = 'foundry-theme';
  const themes = new Set(['dark', 'light']);

  const readCookie = () => {
    const prefix = `${cookieName}=`;
    const value = document.cookie
      .split('; ')
      .find((cookie) => cookie.startsWith(prefix))
      ?.slice(prefix.length);
    return themes.has(value) ? value : 'dark';
  };

  const applyTheme = (theme) => {
    document.documentElement.dataset.theme = theme;
    const nextTheme = theme === 'dark' ? 'light' : 'dark';
    document.querySelectorAll('.theme-toggle').forEach((button) => {
      const label = `Switch to ${nextTheme} mode`;
      button.setAttribute('aria-label', label);
      button.setAttribute('title', label);
      button.setAttribute('aria-pressed', String(theme === 'light'));
    });
  };

  const saveTheme = (theme) => {
    document.cookie = `${cookieName}=${theme}; Max-Age=31536000; Path=/; SameSite=Lax`;
    applyTheme(theme);
  };

  applyTheme(readCookie());

  const init = () => {
    applyTheme(readCookie());
    document.querySelectorAll('.theme-toggle').forEach((button) => {
      button.addEventListener('click', () => {
        const current = document.documentElement.dataset.theme || 'dark';
        saveTheme(current === 'dark' ? 'light' : 'dark');
      });
    });
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init, { once: true });
  } else {
    init();
  }
})();
