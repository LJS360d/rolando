//! Byte-level RNN language model with BPTT training (weights packed as RLM1 blob).

use ndarray::{Array1, Array2};
use rand::seq::SliceRandom;
use rand::Rng;

const MAGIC: &[u8; 4] = b"RLM1";

pub struct Model {
    pub vb: usize,
    pub e: usize,
    pub h: usize,
    pub embed: Array2<f32>,
    pub w_x: Array2<f32>,
    pub w_h: Array2<f32>,
    pub b: Array1<f32>,
    pub w_out: Array2<f32>,
    pub b_out: Array1<f32>,
}

struct Step {
    x: Array1<f32>,
    h_prev: Array1<f32>,
    h: Array1<f32>,
}

pub fn header_size() -> usize {
    16
}

pub fn total_byte_len(vb: usize, e: usize, h: usize) -> usize {
    header_size() + 4 * (vb * e + h * e + h * h + h + vb * h + vb)
}

pub fn create(vb: usize, e: usize, h: usize) -> Vec<u8> {
    let mut rng = rand::thread_rng();
    let limit_x = (6.0f32 / (e + h) as f32).sqrt();
    let limit_o = (6.0f32 / (h + vb) as f32).sqrt();

    let mut embed = Array2::<f32>::zeros((vb, e));
    for v in embed.iter_mut() {
        *v = (rng.gen::<f32>() * 2.0 - 1.0) * limit_x;
    }

    let mut w_x = Array2::<f32>::zeros((h, e));
    for v in w_x.iter_mut() {
        *v = (rng.gen::<f32>() * 2.0 - 1.0) * limit_x;
    }

    let mut w_h = Array2::<f32>::zeros((h, h));
    for v in w_h.iter_mut() {
        *v = (rng.gen::<f32>() * 2.0 - 1.0) * limit_x;
    }

    let b = Array1::<f32>::zeros(h);

    let mut w_out = Array2::<f32>::zeros((vb, h));
    for v in w_out.iter_mut() {
        *v = (rng.gen::<f32>() * 2.0 - 1.0) * limit_o;
    }

    let mut b_out = Array1::<f32>::zeros(vb);
    for v in b_out.iter_mut() {
        *v = (rng.gen::<f32>() * 2.0 - 1.0) * limit_o * 0.1;
    }

    let m = Model {
        vb,
        e,
        h,
        embed,
        w_x,
        w_h,
        b,
        w_out,
        b_out,
    };
    pack(&m)
}

pub fn unpack(bytes: &[u8]) -> Result<Model, String> {
    if bytes.len() < header_size() {
        return Err("truncated header".into());
    }
    if &bytes[0..4] != MAGIC {
        return Err("bad magic".into());
    }
    let vb = u32::from_le_bytes(bytes[4..8].try_into().unwrap()) as usize;
    let e = u32::from_le_bytes(bytes[8..12].try_into().unwrap()) as usize;
    let h = u32::from_le_bytes(bytes[12..16].try_into().unwrap()) as usize;
    let need = total_byte_len(vb, e, h);
    if bytes.len() < need {
        return Err(format!("truncated body: need {} got {}", need, bytes.len()));
    }
    let mut off = header_size();

    let ne = vb * e;
    let embed = Array2::from_shape_vec((vb, e), read_f32s(bytes, &mut off, ne)?)
        .map_err(|e| e.to_string())?;
    let w_x = Array2::from_shape_vec((h, e), read_f32s(bytes, &mut off, h * e)?)
        .map_err(|e| e.to_string())?;
    let w_h = Array2::from_shape_vec((h, h), read_f32s(bytes, &mut off, h * h)?)
        .map_err(|e| e.to_string())?;
    let b = Array1::from(read_f32s(bytes, &mut off, h)?);
    let w_out = Array2::from_shape_vec((vb, h), read_f32s(bytes, &mut off, vb * h)?)
        .map_err(|e| e.to_string())?;
    let b_out = Array1::from(read_f32s(bytes, &mut off, vb)?);

    Ok(Model {
        vb,
        e,
        h,
        embed,
        w_x,
        w_h,
        b,
        w_out,
        b_out,
    })
}

fn read_f32s(bytes: &[u8], off: &mut usize, n: usize) -> Result<Vec<f32>, String> {
    let need = n * 4;
    if *off + need > bytes.len() {
        return Err("oob read".into());
    }
    let mut v = Vec::with_capacity(n);
    for _ in 0..n {
        let b = bytes[*off..*off + 4].try_into().unwrap();
        v.push(f32::from_le_bytes(b));
        *off += 4;
    }
    Ok(v)
}

pub fn pack(m: &Model) -> Vec<u8> {
    let mut out = Vec::with_capacity(total_byte_len(m.vb, m.e, m.h));
    out.extend_from_slice(MAGIC);
    out.extend_from_slice(&(m.vb as u32).to_le_bytes());
    out.extend_from_slice(&(m.e as u32).to_le_bytes());
    out.extend_from_slice(&(m.h as u32).to_le_bytes());
    for x in m.embed.iter() {
        out.extend_from_slice(&x.to_le_bytes());
    }
    for x in m.w_x.iter() {
        out.extend_from_slice(&x.to_le_bytes());
    }
    for x in m.w_h.iter() {
        out.extend_from_slice(&x.to_le_bytes());
    }
    for x in m.b.iter() {
        out.extend_from_slice(&x.to_le_bytes());
    }
    for x in m.w_out.iter() {
        out.extend_from_slice(&x.to_le_bytes());
    }
    for x in m.b_out.iter() {
        out.extend_from_slice(&x.to_le_bytes());
    }
    out
}

#[inline]
fn bucket(token: u32, vb: usize) -> usize {
    (token as usize) % vb
}

fn softmax_inplace(logits: &mut [f32]) {
    if logits.is_empty() {
        return;
    }
    let max = logits.iter().copied().fold(f32::NEG_INFINITY, f32::max);
    let mut s = 0.0f32;
    for z in logits.iter_mut() {
        *z = (*z - max).exp();
        s += *z;
    }
    if s > 1e-12 {
        for z in logits.iter_mut() {
            *z /= s;
        }
    }
}

fn cross_entropy_grad(logits: &[f32], target: usize) -> (f32, Vec<f32>) {
    let mut p: Vec<f32> = logits.to_vec();
    softmax_inplace(&mut p);
    let loss = -p[target].ln().max(-80.0);
    let mut d_logits = vec![0.0f32; logits.len()];
    for (i, d) in d_logits.iter_mut().enumerate() {
        *d = p[i];
    }
    d_logits[target] -= 1.0;
    (loss, d_logits)
}

fn clip_mat2(mut g: Array2<f32>, max_norm: f32) -> Array2<f32> {
    let n: f32 = g.iter().map(|x| x * x).sum::<f32>().sqrt();
    if n > max_norm && n > 1e-12 {
        g *= max_norm / n;
    }
    g
}

fn clip_vec1(mut g: Array1<f32>, max_norm: f32) -> Array1<f32> {
    let n: f32 = g.iter().map(|x| x * x).sum::<f32>().sqrt();
    if n > max_norm && n > 1e-12 {
        g *= max_norm / n;
    }
    g
}

pub fn step_logits(m: &Model, token: u32, h_prev: &Array1<f32>) -> (Array1<f32>, Array1<f32>) {
    let bi = bucket(token, m.vb);
    let x = m.embed.row(bi).to_owned();
    let a = m.w_x.dot(&x) + m.w_h.dot(h_prev) + &m.b;
    let h = a.mapv(|v| v.tanh());
    let logits = m.w_out.dot(&h) + &m.b_out;
    (logits, h)
}

pub fn train_sequences(
    mut m: Model,
    sequences: &[Vec<u32>],
    lr: f32,
    epochs: usize,
    max_seq_len: usize,
    max_norm: f32,
) -> (Vec<u8>, f32) {
    let mut rng = rand::thread_rng();
    let mut seqs: Vec<&Vec<u32>> = sequences.iter().collect();
    if seqs.is_empty() {
        return (pack(&m), 0.0);
    }

    let mut loss_acc = 0.0f32;
    let mut loss_n = 0usize;

    for _ep in 0..epochs {
        seqs.shuffle(&mut rng);
        for seq in &seqs {
            if seq.len() < 2 {
                continue;
            }
            let slen = seq.len().min(max_seq_len);
            let seq = &seq[..slen];

            let mut h = Array1::<f32>::zeros(m.h);
            let mut steps: Vec<(Step, usize)> = Vec::new();

            for t in 0..seq.len() - 1 {
                let bi = bucket(seq[t], m.vb);
                let x = m.embed.row(bi).to_owned();
                let h_prev = h.clone();
                let a = m.w_x.dot(&x) + m.w_h.dot(&h_prev) + &m.b;
                let h_new = a.mapv(|v| v.tanh());
                let tgt = bucket(seq[t + 1], m.vb);
                steps.push((
                    Step {
                        x,
                        h_prev,
                        h: h_new.clone(),
                    },
                    tgt,
                ));
                h = h_new;
            }

            let mut d_embed = Array2::<f32>::zeros((m.vb, m.e));
            let mut d_w_x = Array2::<f32>::zeros((m.h, m.e));
            let mut d_w_h = Array2::<f32>::zeros((m.h, m.h));
            let mut d_b = Array1::<f32>::zeros(m.h);
            let mut d_w_out = Array2::<f32>::zeros((m.vb, m.h));
            let mut d_b_out = Array1::<f32>::zeros(m.vb);

            let mut d_h_next = Array1::<f32>::zeros(m.h);

            for idx in (0..steps.len()).rev() {
                let (ref step, tgt) = steps[idx];
                let logits = m.w_out.dot(&step.h) + &m.b_out;
                let logits_slice: Vec<f32> = logits.to_vec();
                let (l, d_lo) = cross_entropy_grad(&logits_slice, tgt);
                loss_acc += l;
                loss_n += 1;

                let d_lo = Array1::from(d_lo);
                d_b_out = &d_b_out + &d_lo;
                d_w_out = &d_w_out
                    + &Array2::from_shape_fn((m.vb, m.h), |(i, j)| d_lo[i] * step.h[j]);

                let d_h_from_logits = m.w_out.t().dot(&d_lo);
                let d_h_total = &d_h_next + &d_h_from_logits;

                let dtanh = (1.0f32 - step.h.mapv(|v| v * v)) * &d_h_total;
                d_b = &d_b + &dtanh;
                d_w_x = &d_w_x + &Array2::from_shape_fn((m.h, m.e), |(i, j)| dtanh[i] * step.x[j]);
                d_w_h = &d_w_h
                    + &Array2::from_shape_fn((m.h, m.h), |(i, j)| dtanh[i] * step.h_prev[j]);

                let d_x = m.w_x.t().dot(&dtanh);
                d_h_next = m.w_h.t().dot(&dtanh);

                let bi = bucket(seq[idx], m.vb);
                for (i, v) in d_x.iter().enumerate() {
                    d_embed[[bi, i]] += v;
                }
            }

            d_embed = clip_mat2(d_embed, max_norm);
            d_w_x = clip_mat2(d_w_x, max_norm);
            d_w_h = clip_mat2(d_w_h, max_norm);
            d_b = clip_vec1(d_b, max_norm);
            d_w_out = clip_mat2(d_w_out, max_norm);
            d_b_out = clip_vec1(d_b_out, max_norm);

            m.embed -= &(&d_embed * lr);
            m.w_x -= &(&d_w_x * lr);
            m.w_h -= &(&d_w_h * lr);
            m.b -= &(&d_b * lr);
            m.w_out -= &(&d_w_out * lr);
            m.b_out -= &(&d_b_out * lr);
        }
    }

    let avg = if loss_n > 0 {
        loss_acc / loss_n as f32
    } else {
        0.0
    };
    (pack(&m), avg)
}
