function showSnackbar(message, alertClass) {
  const toast = document.createElement('div');
  toast.classList.add('toast', 'toast-end');

  const alert = document.createElement('div');
  alert.classList.add('alert', alertClass);

  const span = document.createElement('span');
  span.textContent = message;

  alert.appendChild(span);
  toast.appendChild(alert);

  document.body.appendChild(toast);


  setTimeout(() => {
    document.body.removeChild(toast);
  }, 2000);
}