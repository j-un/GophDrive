function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

export function defaultNoteName(now: Date): string {
  const year = String(now.getFullYear());
  const month = pad2(now.getMonth() + 1);
  const day = pad2(now.getDate());
  const hour = pad2(now.getHours());
  const minute = pad2(now.getMinutes());
  const second = pad2(now.getSeconds());
  return `${year}-${month}-${day} ${hour}:${minute}:${second}`;
}
