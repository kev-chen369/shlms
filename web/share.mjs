// This module receives an access token from the signed-in H5 session. It never stores it.
export async function copyPromotionShare({ requestId, artifactType, scene, accessToken, clipboard = navigator.clipboard, request = fetch, eventId = crypto.randomUUID() }) {
  if (!/^[A-Za-z0-9_-]{1,128}$/.test(requestId) || !['link', 'text'].includes(artifactType) || !/^[A-Za-z0-9_-]{1,80}$/.test(scene) || !/^[A-Za-z0-9_-]{1,128}$/.test(eventId) || typeof accessToken !== 'string' || !accessToken) {
    throw new Error('INVALID_SHARE_REQUEST');
  }
  const base = `/api/v1/promotions/convert/${encodeURIComponent(requestId)}`;
  const headers = { Authorization: `Bearer ${accessToken}` };
  const artifactResponse = await request(`${base}/share-artifacts?type=${artifactType}`, { headers, cache: 'no-store' });
  if (!artifactResponse.ok) return { copied: false, reported: false, reason: 'ARTIFACT_UNAVAILABLE' };
  const artifact = (await artifactResponse.json())?.data;
  if (artifact?.requestId !== requestId || artifact?.type !== artifactType || typeof artifact?.content !== 'string' || !artifact.content) {
    return { copied: false, reported: false, reason: 'ARTIFACT_INVALID' };
  }
  try {
    await clipboard.writeText(artifact.content);
  } catch {
    return { copied: false, reported: false, reason: 'COPY_FAILED' };
  }
  try {
    const response = await request(`${base}/share-events`, {
      method: 'POST', headers: { ...headers, 'Content-Type': 'application/json' }, cache: 'no-store',
      body: JSON.stringify({ eventId, artifactType, scene, action: 'COPY_REPORTED' }),
    });
    if (!response.ok) return { copied: true, reported: false, reason: 'REPORT_FAILED' };
    const report = (await response.json())?.data;
    if (report?.eventId !== eventId || report?.requestId !== requestId || report?.action !== 'COPY_REPORTED') {
      return { copied: true, reported: false, reason: 'REPORT_FAILED' };
    }
    return { copied: true, reported: true, eventId };
  } catch {
    return { copied: true, reported: false, reason: 'REPORT_FAILED' };
  }
}
