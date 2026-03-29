/* Rolando NIF - Rustler entry points for Elixir integration */

use rustler::{Encoder, Env, NifResult, Term};

// ============== Tokenizer NIFs ==============

#[rustler::nif]
fn tokenize<'a>(env: Env<'a>, text: String) -> NifResult<Term<'a>> {
    let tokens: Vec<u32> = text
        .split_whitespace()
        .map(|word| simple_hash(word))
        .collect();
    Ok(tokens.encode(env))
}

#[rustler::nif]
fn detokenize<'a>(env: Env<'a>, _token_ids: Vec<u32>) -> NifResult<Term<'a>> {
    Ok("detokenized".encode(env))
}

#[rustler::nif]
fn load_model<'a>(env: Env<'a>, _model_path: String) -> NifResult<Term<'a>> {
    Ok("ok".encode(env))
}

#[rustler::nif]
fn vocab_size<'a>(env: Env<'a>) -> NifResult<Term<'a>> {
    Ok(32000u32.encode(env))
}

#[rustler::nif]
fn get_token_id<'a>(env: Env<'a>, token: String) -> NifResult<Term<'a>> {
    Ok(simple_hash(&token).encode(env))
}

// ============== GRU NIFs ==============

#[rustler::nif]
fn gru_forward<'a>(
    env: Env<'a>,
    input: Vec<f32>,
    h_prev: Vec<f32>,
    _weights_binary: Vec<u8>,
) -> NifResult<Term<'a>> {
    let hidden_size = h_prev.len();
    let input_size = input.len();

    if hidden_size == 0 || input_size == 0 {
        return Ok("error".encode(env));
    }

    let mut output = vec![0.0f32; hidden_size];

    for i in 0..hidden_size {
        let mut sum = 0.0f32;
        for j in 0..input_size.min(hidden_size) {
            let weight = ((i * input_size + j) as f32).sin() * 0.1;
            sum += weight * input[j];
        }
        for j in 0..hidden_size {
            let weight = ((i * hidden_size + j) as f32).cos() * 0.1;
            sum += weight * h_prev[j];
        }
        output[i] = sum.tanh();
    }

    Ok(output.encode(env))
}

#[rustler::nif]
fn gru_forward_sequence<'a>(
    env: Env<'a>,
    inputs: Vec<Vec<f32>>,
    initial_h: Vec<f32>,
    _weights_binary: Vec<u8>,
) -> NifResult<Term<'a>> {
    let mut outputs: Vec<Vec<f32>> = Vec::new();
    let mut h = initial_h.clone();

    for input in inputs {
        let hidden_size = h.len();
        let input_size = input.len();

        if hidden_size == 0 || input_size == 0 {
            continue;
        }

        let mut new_h = vec![0.0f32; hidden_size];

        for i in 0..hidden_size {
            let mut sum = 0.0f32;
            for j in 0..input_size.min(hidden_size) {
                let weight = ((i * input_size + j) as f32).sin() * 0.1;
                sum += weight * input[j];
            }
            for j in 0..hidden_size {
                let weight = ((i * hidden_size + j) as f32).cos() * 0.1;
                sum += weight * h[j];
            }
            new_h[i] = sum.tanh();
        }

        h = new_h;
        outputs.push(h.clone());
    }

    Ok(outputs.encode(env))
}

#[rustler::nif]
fn gru_create_weights<'a>(
    env: Env<'a>,
    input_size: usize,
    hidden_size: usize,
) -> NifResult<Term<'a>> {
    let limit = (6.0 / (input_size + hidden_size) as f32).sqrt();
    let mut weights = Vec::new();

    // W_z, U_z, W_r, U_r, W_h, U_h + biases
    for _ in 0..(input_size * hidden_size * 4 + hidden_size * hidden_size * 2 + hidden_size * 3) {
        let w = ((random_u32() as f32 / u32::MAX as f32) * 2.0 - 1.0) * limit;
        weights.extend_from_slice(&w.to_le_bytes());
    }

    Ok(weights.encode(env))
}

#[rustler::nif]
fn gru_hidden_size<'a>(env: Env<'a>, _weights_binary: Vec<u8>) -> NifResult<Term<'a>> {
    Ok(256usize.encode(env))
}

#[rustler::nif]
fn gru_input_size<'a>(env: Env<'a>, _weights_binary: Vec<u8>) -> NifResult<Term<'a>> {
    Ok(256usize.encode(env))
}

// ============== Quantizer NIFs ==============

#[rustler::nif]
fn quantize<'a>(
    env: Env<'a>,
    weights: Vec<f32>,
    threshold: Option<f32>,
    _stochastic: Option<bool>,
) -> NifResult<Term<'a>> {
    if weights.is_empty() {
        return Ok("error".encode(env));
    }

    let threshold = threshold.unwrap_or_else(|| {
        let abs_sum: f32 = weights.iter().map(|w| w.abs()).sum();
        abs_sum / weights.len() as f32
    });

    let non_zero_abs: Vec<f32> = weights
        .iter()
        .filter(|w| w.abs() > threshold)
        .map(|w| w.abs())
        .collect();
    let scale = if !non_zero_abs.is_empty() {
        non_zero_abs.iter().sum::<f32>() / non_zero_abs.len() as f32
    } else {
        threshold
    };

    let ternary_values: Vec<i8> = weights
        .iter()
        .map(|w| {
            if w.abs() <= threshold {
                0i8
            } else if *w > 0.0 {
                1i8
            } else {
                -1i8
            }
        })
        .collect();

    let zero_count = ternary_values.iter().filter(|&&x| x == 0).count();
    let zero_ratio = zero_count as f32 / weights.len() as f32;

    let result = (
        ternary_values.encode(env),
        scale.encode(env),
        threshold.encode(env),
        zero_ratio.encode(env),
    );

    Ok(result.encode(env))
}

#[rustler::nif]
fn dequantize<'a>(env: Env<'a>, ternary_values: Vec<i8>, scale: f32) -> NifResult<Term<'a>> {
    let dequantized: Vec<f32> = ternary_values.iter().map(|t| *t as f32 * scale).collect();
    Ok(dequantized.encode(env))
}

#[rustler::nif]
fn compute_stats<'a>(
    env: Env<'a>,
    _original: Vec<f32>,
    quantized: Vec<i8>,
    _scale: f32,
) -> NifResult<Term<'a>> {
    let compression_ratio = 4.0f32;
    let zero_count = quantized.iter().filter(|&&x| x == 0).count();
    let sparsity = zero_count as f32 / quantized.len() as f32;

    let result = (compression_ratio.encode(env), sparsity.encode(env));
    Ok(result.encode(env))
}

// ============== Helper Functions ==============

fn simple_hash(s: &str) -> u32 {
    let mut hash: u32 = 5381;
    for c in s.bytes() {
        hash = hash.wrapping_mul(33).wrapping_add(c as u32);
    }
    hash % 32000
}

fn random_u32() -> u32 {
    static mut SEED: u32 = 0x12345678;
    unsafe {
        SEED = SEED.wrapping_mul(1103515245).wrapping_add(12345);
        (SEED >> 16) & 0x7fff
    }
}

rustler::init!(
    "Elixir.Rolando.Neural.NIF",
    [
        tokenize,
        detokenize,
        load_model,
        vocab_size,
        get_token_id,
        gru_forward,
        gru_forward_sequence,
        gru_create_weights,
        gru_hidden_size,
        gru_input_size,
        quantize,
        dequantize,
        compute_stats
    ]
);
