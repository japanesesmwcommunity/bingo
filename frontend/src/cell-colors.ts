import type { Player } from './types';

// On a square, rays from the center to (100%, 5/6 height) and
// (0%, 5/6 height) form three regions of exactly one third of its area.
const THREE_WAY_ANGLE = 90 + (Math.atan(2 / 3) * 180) / Math.PI;

export function cellFill(owners: Pick<Player, 'color'>[]): string | undefined {
  const colors = owners.map((owner) => owner.color);
  switch (colors.length) {
    case 0:
      return undefined;
    case 1:
      return colors[0];
    case 2:
      return `linear-gradient(135deg, ${colors[0]} 0% 50%, ${colors[1]} 50% 100%)`;
    case 3:
      return `conic-gradient(${colors[0]} 0deg ${THREE_WAY_ANGLE}deg, ${colors[1]} ${THREE_WAY_ANGLE}deg ${360 - THREE_WAY_ANGLE}deg, ${colors[2]} ${360 - THREE_WAY_ANGLE}deg 360deg)`;
    default:
      return `conic-gradient(${colors[0]} 0deg 90deg, ${colors[1]} 90deg 180deg, ${colors[2]} 180deg 270deg, ${colors[3]} 270deg 360deg)`;
  }
}
