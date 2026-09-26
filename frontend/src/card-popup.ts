export const CARD_POPUP_FEATURES = 'popup,width=640,height=640,resizable=yes,scrollbars=yes';

export function openCardPopup(roomId: string, host: Pick<Window, 'open'> = window): boolean {
  const popup = host.open(
    `/?view=card#room=${encodeURIComponent(roomId)}`,
    `bingo-card-${roomId}`,
    CARD_POPUP_FEATURES,
  );
  if (!popup) return false;
  popup.focus();
  return true;
}
