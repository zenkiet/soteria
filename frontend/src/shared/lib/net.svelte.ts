export const net = $state({ offline: false, back: false });
export const OFFLINE = "can't reach the server";
export const lost = () => (net.offline = true);
