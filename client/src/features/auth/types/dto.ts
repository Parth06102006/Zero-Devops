export interface CurrentUserDto {
  id: string;
  provider: "github";
  username: string;
  email: string;
  avatarURL: string;
}

export type AuthResponseDto = CurrentUserDto;
