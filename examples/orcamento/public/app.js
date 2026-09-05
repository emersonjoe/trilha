// Submit the month selector on change (the "Ver" button still works without JS).
document.addEventListener("change", (e) => {
  const s = e.target.closest("select[data-submit]");
  if (s) s.form.requestSubmit();
});
// Reopen the dialog when the page came back with validation errors inside it.
document.querySelectorAll("dialog .ui-field-error").forEach((el) => el.closest("dialog")?.showModal());
