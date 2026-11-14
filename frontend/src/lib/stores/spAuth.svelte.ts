/**
 * S P Reseller portal state store using Svelte 5 runes.
 * Backed by real Go Fiber API (/api/v1/sp).
 */

import { spApi, SpApiError } from '$lib/sp-api.js';

export interface SpUser {
	id: number;
	name: string;
	email: string;
	phone?: string;
	balance: number;
	type: string;
	status: string;
	is_owner?: boolean;
	permissions?: string[];
}

interface SpState {
	user: SpUser | null;
	loading: boolean;
	initialized: boolean;
}

function createSpStore() {
	let state = $state<SpState>({
		user: null,
		loading: false,
		initialized: false
	});

	return {
		get user() {
			return state.user;
		},
		get loading() {
			return state.loading;
		},
		get initialized() {
			return state.initialized;
		},
		get isLoggedIn() {
			return state.user !== null;
		},
		get balance() {
			return state.user?.balance ?? 0;
		},
		get isOwner() {
			return state.user?.is_owner === true;
		},
		can(permission: string) {
			return state.user?.is_owner === true || (state.user?.permissions ?? []).includes(permission);
		},

		async init() {
			if (state.initialized) return;
			state.loading = true;

			const token = spApi.getToken();
			if (!token) {
				state.user = null;
				state.initialized = true;
				state.loading = false;
				return;
			}

			try {
				const res = await spApi.get<{ data: SpUser; is_owner?: boolean; permissions?: string[] }>('/api/v1/sp/auth/me');
				state.user = { ...res.data, is_owner: res.is_owner, permissions: res.permissions ?? [] };
			} catch (e) {
				if (e instanceof SpApiError && e.status === 401) {
					spApi.clearToken();
					state.user = null;
				}
			} finally {
				state.loading = false;
				state.initialized = true;
			}
		},

		async login(email: string, password: string, remember: boolean = true) {
			state.loading = true;
			try {
				const res = await spApi.post<{ data: { token: string; user: SpUser } }>('/api/v1/sp/auth/login', {
					email,
					password,
					remember
				});

				if (res?.data?.token) {
					spApi.setToken(res.data.token, remember);
				}

				try {
					const meRes = await spApi.get<{ data: SpUser; is_owner?: boolean; permissions?: string[] }>('/api/v1/sp/auth/me');
					state.user = { ...meRes.data, is_owner: meRes.is_owner, permissions: meRes.permissions ?? [] };
				} catch {
					state.user = res?.data?.user ?? null;
				}

				return { success: true };
			} catch (e) {
				spApi.clearToken();
				state.user = null;
				if (e instanceof SpApiError) {
					const message =
						(e.data?.message as string) ??
						((e.data?.errors as Record<string, string[]>)?.email?.[0] as string) ??
						'Invalid Reseller credentials.';
					return { success: false, message };
				}
				return { success: false, message: 'Network error. Please try again.' };
			} finally {
				state.loading = false;
			}
		},

		async logout() {
			try {
				await spApi.post('/api/v1/sp/auth/logout');
			} catch {
				// ignore
			} finally {
				spApi.clearToken();
				state.user = null;
				state.initialized = false;
			}
		},

		async refreshUser() {
			try {
				const res = await spApi.get<{ data: SpUser; is_owner?: boolean; permissions?: string[] }>('/api/v1/sp/auth/me');
				state.user = { ...res.data, is_owner: res.is_owner, permissions: res.permissions ?? [] };
			} catch {
				// ignore
			}
		},

		updateBalance(newBalance: number) {
			if (state.user) {
				state.user = { ...state.user, balance: newBalance };
			}
		}
	};
}

export const spAuth = createSpStore();
