import type { CurrentUserDto } from "@/features/auth/types/dto";
import type { AuthUser, CurrentUser } from "@/types/auth";

export function mapCurrentUser(dto: CurrentUserDto): CurrentUser {
  const user: AuthUser = {
    id: dto.id,
    displayName: dto.username,
    username: dto.username,
    email: dto.email || null,
    avatarUrl: dto.avatarURL || null,
    provider: dto.provider,
  };
  return { user };
}
