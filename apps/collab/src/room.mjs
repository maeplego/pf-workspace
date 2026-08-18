const ULID = /^[0-9A-HJKMNP-TV-Z]{26}$/i;

export function validRoom(name) {
  return typeof name === "string" && ULID.test(name);
}

export function ticketMatchesRoom(ticketDocId, documentName) {
  return ticketDocId === documentName && validRoom(documentName);
}
