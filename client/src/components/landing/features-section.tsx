import { valueProps } from "./data/landing-content";

export function FeaturesSection() {
  return (
    <section className="border-y border-white/[0.06] bg-white/[0.015]">
      <div className="mx-auto grid max-w-7xl gap-0 px-5 lg:grid-cols-3 lg:px-8">
        {valueProps.map(({ number, title, description }, index) => (
          <div
            key={title}
            className={`p-7 ${
              index < 2 ? "border-b border-white/[0.06] lg:border-b-0 lg:border-r" : ""
            }`}
          >
            <p className="text-xs text-white/25">{number}</p>
            <h2 className="mt-5 text-lg font-medium">{title}</h2>
            <p className="mt-2 text-sm leading-6 text-white/35">{description}</p>
          </div>
        ))}
      </div>
    </section>
  );
}
