const workflowItems = [
  "GitHub repository",
  "Project configuration",
  "Build queue",
  "Worker build",
  "Deployment",
] as const;

export function AuthContextPanel() {
  return (
    <section className="relative hidden min-h-[620px] overflow-hidden lg:block">
      <div className="absolute inset-0 mesh-glow" />
      <div className="absolute inset-8 rounded-3xl border border-white/[0.07] bg-white/[0.02] p-8">
        <p className="font-mono text-[10px] uppercase tracking-[.18em] text-white/25">
          Repository → deployment
        </p>
        <div className="mt-12 space-y-3">
          {workflowItems.map((item, i) => (
            <div
              key={item}
              className="flex items-center gap-4 rounded-2xl border border-white/[0.07] bg-black/30 p-4"
            >
              <span className="font-mono text-[10px] text-white/20">0{i + 1}</span>
              <span className="text-sm text-white/65">{item}</span>
              <span className="ml-auto size-2 rounded-full bg-gradient-to-r from-cyan-300 to-fuchsia-300" />
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
