import os
from PIL import Image

def create_gif():
    images_dir = r"B:\workgit\queuelens\docs\images"
    output_gif = os.path.join(images_dir, "queuelens_demo.gif")
    
    frame_files = [
        os.path.join(images_dir, "queuelens-1.png"),
        os.path.join(images_dir, "queuelens-2.png"),
        os.path.join(images_dir, "queuelens-3.png"),
        os.path.join(images_dir, "queuelens-4.png"),
    ]
    
    frames = []
    # Target 1280px width for clean sharpness
    target_width = 1280
    
    for f in frame_files:
        if not os.path.exists(f):
            print(f"Error: {f} does not exist")
            return
        img = Image.open(f)
        aspect = img.size[1] / img.size[0]
        target_height = int(target_width * aspect)
        resized = img.convert("RGB").resize((target_width, target_height), Image.Resampling.LANCZOS)
        # Quantize to 128 colors with Floyd-Steinberg dithering for smooth gradients and compact size
        quantized = resized.quantize(colors=128, method=Image.Quantize.MEDIANCUT, dither=Image.Dither.FLOYDSTEINBERG)
        frames.append(quantized)
    
    # Save as animated GIF: 2200ms per screen for comfortable reading
    frames[0].save(
        output_gif,
        save_all=True,
        append_images=frames[1:],
        duration=2200,
        loop=0,
        optimize=True
    )
    
    size_kb = os.path.getsize(output_gif) / 1024
    print(f"Created {output_gif} ({size_kb:.1f} KB)")

if __name__ == "__main__":
    create_gif()
