// Lowercases and strips accents, so "res" finds "Résistance".
export function fold(text: string): string {
  return text
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
}
