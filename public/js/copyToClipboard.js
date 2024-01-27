function copyToClipboard(text, successMsg) {
  const input = document.createElement('input');
  input.value = text;
  document.body.appendChild(input);
  input.select();

  try {
    const successful = document.execCommand('copy');
    if (successful) {

      showSnackbar(`Copied ${successMsg} to clipboard!`, 'alert-success');
    } else {
      throw new Error('Unable to copy. Your browser may not support it.');
    }
  } catch (err) {

    showSnackbar(err.message, 'alert-error');
  } finally {
    document.body.removeChild(input);
  }
}
