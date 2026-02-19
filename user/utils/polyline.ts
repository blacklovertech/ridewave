/**
 * Decodes an encoded path string into a sequence of LatLngs.
 * Adapted from Google Maps Polyline Encoding/Decoding algorithm.
 */
export const decode = (encodedPath: string) => {
  const len = encodedPath.length || 0;
  const path = [];
  let index = 0;
  let lat = 0;
  let lng = 0;

  while (index < len) {
    let result = 1;
    let shift = 0;
    let b;
    do {
      b = encodedPath.charCodeAt(index++) - 63 - 1;
      result += b << shift;
      shift += 5;
    } while (b >= 0x1f);
    lat += result & 1 ? ~(result >> 1) : result >> 1;

    result = 1;
    shift = 0;
    do {
      b = encodedPath.charCodeAt(index++) - 63 - 1;
      result += b << shift;
      shift += 5;
    } while (b >= 0x1f);
    lng += result & 1 ? ~(result >> 1) : result >> 1;

    path.push([lat * 1e-5, lng * 1e-5]);
  }
  return path;
};
