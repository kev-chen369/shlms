import type { InjectionKey, Ref } from 'vue'
// Supplied by the real identity owner when integrated; never issued by orders UI.
export const promoterSessionKey: InjectionKey<Ref<string | null>> = Symbol('promoter-session')
