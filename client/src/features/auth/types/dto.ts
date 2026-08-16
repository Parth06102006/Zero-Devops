/**
 * Raw wire shapes returned by the (not-yet-finalized) backend auth endpoints.
 *
 * BACKEND INTEGRATION POINT: these types are placeholders reflecting a
 * reasonable guess at the contract. When the real backend response shape is
 * confirmed, update these DTOs and `auth-mapper.ts` — nothing outside this
 * folder should need to change, since the rest of the app only ever sees the
 * domain types in `types/auth.ts`.
 */

export interface AuthUserDto {
  id: string;
  display_name: string;
  username: string;
  email: string | null;
  avatar_url: string | null;
  provider: "github";
  created_at: string;
}

export interface SessionDto {
  id: string;
  created_at: string;
  expires_at: string;
  refreshable: boolean;
}

export interface CurrentUserDto {
  user: AuthUserDto;
  session: SessionDto;
}

export type AuthResponseDto = CurrentUserDto;
