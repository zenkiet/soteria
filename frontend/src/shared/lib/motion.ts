import { gsap } from 'gsap';

gsap.defaults({ duration: 0.3, ease: 'power2.out' });

const MOTION = '(prefers-reduced-motion: no-preference)';

function action(build: (node: HTMLElement) => void) {
	return (node: HTMLElement) => {
		const mm = gsap.matchMedia();
		mm.add(MOTION, () => build(node));
		return { destroy: () => mm.revert() };
	};
}

export const reveal = action((n) =>
	gsap.from(n.children, { autoAlpha: 0, y: 8, stagger: 0.03, clearProps: 'all' })
);
export const pop = action((n) => gsap.from(n, { autoAlpha: 0, scale: 0.97, y: 4, duration: 0.18 }));
export const slideIn = action((n) => gsap.from(n, { autoAlpha: 0, x: 16 }));
