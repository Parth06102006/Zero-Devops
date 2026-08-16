import type { AuthUserDto, CurrentUserDto, SessionDto } from "@/features/auth/types/dto";
import type { AuthUser, CurrentUser, Session } from "@/types/auth";

/**
 * BACKEND INTEGRATION POINT: this is the translation boundary between
 * whatever the backend actually sends and the stable domain shape the rest
 * of the app consumes. If field names, casing, or nesting change on the
 * backend, fix it here only.
 */
export function mapAuthUser(dto: AuthUserDto): AuthUser {
  return {
    id: dto.id,
    displayName: dto.display_name,
    username: dto.username,
    email: dto.email,
    avatarUrl: dto.avatar_url,
    provider: dto.provider,
    createdAt: dto.created_at,
  };
}

export function mapSession(dto: SessionDto): Session {
  return {
    id: dto.id,
    createdAt: dto.created_at,
    tokens: {
      expiresAt: dto.expires_at,
      refreshable: dto.refreshable,
    },
  };
}

export function mapCurrentUser(dto: CurrentUserDto): CurrentUser {
  return {
    user: mapAuthUser(dto.user),
    session: mapSession(dto.session),
  };
}
