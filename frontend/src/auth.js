// Shared BasaltPass OAuth (PKCE) client.
// Configure per app via .env:
//   VITE_BASALTPASS_URL=http://localhost:8101
//   VITE_BASALTPASS_CLIENT_ID=your-client-id
//   VITE_BASALTPASS_REDIRECT_URI=http://localhost:5173/auth/callback
// The app backend must expose POST /api/auth/exchange (code + code_verifier + redirect_uri)
// which mints the session from BasaltPass' token endpoint.

const verifierKey = (app) => `${app}_pkce_verifier`
const stateKey = (app) => `${app}_oauth_state`
const tokenKey = (app) => `${app}_access_token`
const idTokenKey = (app) => `${app}_id_token`

const baseURL = (import.meta.env.VITE_BASALTPASS_URL || 'http://localhost:8101').replace(/\/$/, '')
const clientID = import.meta.env.VITE_BASALTPASS_CLIENT_ID || ''
const redirectURI = import.meta.env.VITE_BASALTPASS_REDIRECT_URI || `${window.location.origin}/auth/callback`

function randomString(bytes) {
  const value = new Uint8Array(bytes)
  crypto.getRandomValues(value)
  return btoa(String.fromCharCode(...value)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

async function challenge(verifier) {
  const encoded = new TextEncoder().encode(verifier)
  const digest = await crypto.subtle.digest('SHA-256', encoded)
  return btoa(String.fromCharCode(...new Uint8Array(digest))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

export async function beginLogin(app = 'app') {
  if (!clientID) throw new Error('BasaltPass OAuth client is not configured')
  const verifier = randomString(64)
  const state = randomString(20)
  sessionStorage.setItem(verifierKey(app), verifier)
  sessionStorage.setItem(stateKey(app), state)
  const params = new URLSearchParams({
    response_type: 'code',
    client_id: clientID,
    redirect_uri: redirectURI,
    scope: 'openid profile email',
    state,
    code_challenge: await challenge(verifier),
    code_challenge_method: 'S256',
  })
  window.location.assign(`${baseURL}/api/v1/oauth/authorize?${params}`)
}

export async function finishLogin(app = 'app', search = window.location.search) {
  const params = new URLSearchParams(search)
  const code = params.get('code')
  const state = params.get('state')
  const verifier = sessionStorage.getItem(verifierKey(app))
  if (!code || !verifier || state !== sessionStorage.getItem(stateKey(app))) {
    throw new Error('BasaltPass authorization response is invalid')
  }
  const response = await fetch('/api/auth/exchange', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code, code_verifier: verifier, redirect_uri: redirectURI }),
  })
  const data = await response.json()
  if (!response.ok || !data.access_token) throw new Error(data.error || 'BasaltPass authorization failed')
  sessionStorage.removeItem(verifierKey(app))
  sessionStorage.removeItem(stateKey(app))
  localStorage.setItem(tokenKey(app), data.access_token)
  if (data.id_token) localStorage.setItem(idTokenKey(app), data.id_token)
  return data
}

export function isLoggedIn(app = 'app') {
  return Boolean(localStorage.getItem(tokenKey(app)))
}

export function logout(app = 'app') {
  localStorage.removeItem(tokenKey(app))
  localStorage.removeItem(idTokenKey(app))
}

export function userProfile(app = 'app') {
  const token = localStorage.getItem(idTokenKey(app))
  if (!token) return { name: 'BasaltPass 用户', email: '', picture: '' }
  try {
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
    return {
      name: payload.name || payload.preferred_username || payload.nickname || payload.email || 'BasaltPass 用户',
      email: payload.email || '',
      picture: payload.picture || '',
    }
  } catch {
    return { name: 'BasaltPass 用户', email: '', picture: '' }
  }
}
