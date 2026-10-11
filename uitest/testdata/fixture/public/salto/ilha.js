// The island of /fluxos/ilha/salto: a button that posts to a route.go that
// redirects. island.post turns the redirect into a navigation (#302).
import { label } from "./motor.js";

export default (el, props, island) => {
  const b = document.createElement("button");
  b.type = "button";
  b.id = "concluir";
  b.textContent = label;
  b.addEventListener("click", () => island.post(props.to));
  el.replaceChildren(b);
};
