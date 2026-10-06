// Counts how many times this file ran in the document: a client navigation
// runs a script it brings once per URL, never once per page (#290).
window.__runs = (window.__runs || 0) + 1;
