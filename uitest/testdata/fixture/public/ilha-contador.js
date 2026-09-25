// The island of the fixture: a counter that starts at props.start.
export default (el, props) => {
  let n = props.start;
  const b = document.createElement("button");
  b.type = "button";
  b.id = "contador";
  const show = () => { b.textContent = "count " + n; };
  b.addEventListener("click", () => { n++; show(); });
  show();
  el.replaceChildren(b);
};
