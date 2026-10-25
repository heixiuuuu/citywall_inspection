/** 登录凭据与用户信息的本地存储。 */

const TOKEN_KEY = 'city_wall_token'
const USER_KEY = 'city_wall_user'

export function setToken(token) {
  uni.setStorageSync(TOKEN_KEY, token)
}

export function getToken() {
  return uni.getStorageSync(TOKEN_KEY) || ''
}

export function clearToken() {
  uni.removeStorageSync(TOKEN_KEY)
  uni.removeStorageSync(USER_KEY)
}

export function setUser(user) {
  uni.setStorageSync(USER_KEY, user)
}

export function getUser() {
  return uni.getStorageSync(USER_KEY) || null
}

export function isLoggedIn() {
  return Boolean(getToken())
}
