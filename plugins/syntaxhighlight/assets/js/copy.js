function foundryShCopy(btn) {
  var wrapper = btn.closest('.sh-wrapper');
  if (!wrapper) return;

  var pre = wrapper.querySelector('pre');
  if (!pre) return;

  var text = pre.textContent || '';

  if (!navigator.clipboard) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    try { document.execCommand('copy'); } catch (_) {}
    document.body.removeChild(ta);
    markCopied(btn);
    return;
  }

  navigator.clipboard.writeText(text).then(function () {
    markCopied(btn);
  });
}

function markCopied(btn) {
  var original = btn.textContent;
  btn.textContent = 'Copied!';
  btn.classList.add('sh-copied');
  setTimeout(function () {
    btn.textContent = original;
    btn.classList.remove('sh-copied');
  }, 1800);
}
