
import numpy as np
import json
from multiprocessing import Pool, cpu_count
from tqdm import tqdm
import collections

# ================= 配置区 =================
# Go 版见同目录 demosim/:  go run ./demosim -mode BASE -n 10000000
SIM_MODE = "BASE"  # 切换模式: "BASE" 或 "BUY"
TOTAL_COUNT = 10000000

class SweetWildSimulator:
    def __init__(self, config):
        self.cfg = config
        self.g_cfg = config['game_config']
        self.m_configs = config['modes_config']
        self.payouts = config['symbol_payouts']
        # 预载所有卷轴
        self.all_reels = {k: [list(s) for s in v] for k, v in config['reel_sets'].items()}

        self.cols, self.rows = self.g_cfg['columns'], self.g_cfg['rows']
        self.wild_id = self.g_cfg['wild_id']
        self.sc_id = self.g_cfg['sc_id']
        self.m_ball_id = self.g_cfg['multiplier_id']
        self.base_bet = self.g_cfg['base_bet']
        self.wild_max = self.g_cfg['wild_max_limit']
        self.max_win_cap = self.g_cfg.get('max_win_cap', 20000.0)

    def select_reels(self, m_conf, rng):
        r_configs = m_conf.get('reel_configs', [])
        if not r_configs:
            return self.all_reels[m_conf['reel_key']]
        keys = [r['reel_key'] for r in r_configs]
        weights = np.array([r['weight'] for r in r_configs], dtype=float)
        weights /= weights.sum()
        chosen_key = rng.choice(keys, p=weights)
        return self.all_reels[chosen_key]

    def _get_mode_params(self, mode_key, rng):
        m_conf = self.m_configs[mode_key]
        m_data = m_conf['multiplier']
        m_vals = np.array([int(k) for k in m_data['weight'].keys()])
        m_weights = np.array(list(m_data['weight'].values()), dtype=float)
        m_weights /= m_weights.sum()

        return {
            'reels': self.select_reels(m_conf, rng),
            'm_prob': m_data['prob_per_col'],
            'm_vals': m_vals,
            'm_weights': m_weights,
            'wild_init': m_conf['wild_gen']['initial_spawn'],
            'wild_refill': m_conf['wild_gen']['tumble_refill']
        }

    def run_one_spin(self, mode_key, rng):
        p = self._get_mode_params(mode_key, rng)
        reels = p['reels']
        top_indices = []
        grid = np.zeros((self.cols, self.rows), dtype=int)

        for c in range(self.cols):
            r_len = len(reels[c])
            t_idx = rng.integers(0, r_len)
            top_indices.append(t_idx)
            for r in range(self.rows): grid[c, r] = reels[c][(t_idx + r) % r_len]

        res = {
            'pure_win': 0.0, 'wild_win': 0.0, 'sc_count': 0, 'win_m_total': 0,
            'is_max_win': False, 'm_ball_count': 0, 'tumbles': 0,
            'wild_count': 0, 'wild_explodes': 0,
            'symbol_raw_wins': collections.defaultdict(float)
        }
        m_values_map = {}

        curr_w = np.count_nonzero(grid == self.wild_id)
        if curr_w < self.wild_max:
            w_cfg = p['wild_init']
            target = rng.choice([int(k) for k in w_cfg.keys()], p=np.array(list(w_cfg.values())) / sum(w_cfg.values()))
            eligible = [(c, r) for c in range(self.cols) for r in range(self.rows) if
                        grid[c, r] not in [self.sc_id, self.m_ball_id, self.wild_id]]
            if len(eligible) > 0 and target > 0:
                for idx in rng.choice(len(eligible), size=min(target, self.wild_max - curr_w, len(eligible)),
                                      replace=False):
                    grid[eligible[idx]] = self.wild_id

        res['wild_count'] = np.count_nonzero(grid == self.wild_id)

        for c in range(self.cols):
            if rng.random() < p['m_prob']:
                v_rows = [r for r in range(self.rows) if grid[c, r] not in [self.sc_id, self.wild_id]]
                if v_rows:
                    tr = rng.choice(v_rows)
                    grid[c, tr], m_values_map[(c, tr)] = self.m_ball_id, rng.choice(p['m_vals'], p=p['m_weights'])

        spin_p, spin_w = 0.0, 0.0
        while True:
            counts = collections.Counter(grid.flatten())
            wild_count, mask = counts[self.wild_id], np.zeros_like(grid, dtype=bool)
            p_step, w_step, is_win, is_explode = 0.0, 0.0, False, False

            for s_id_str, pays in self.payouts.items():
                s_id = int(s_id_str)
                if s_id in [self.sc_id, self.m_ball_id, self.wild_id]: continue
                total = counts[s_id] + wild_count
                if total >= 8:
                    is_win = True
                    win_val = pays.get("8-9" if total <= 9 else ("10-11" if total <= 11 else "12+"), 0)
                    mask[grid == s_id] = True
                    res['symbol_raw_wins'][s_id] += win_val
                    if wild_count > 0: w_step += win_val
                    else: p_step += win_val

            if not is_win and mode_key != 'base_game' and wild_count > 0:
                is_explode = True
                res['wild_explodes'] += 1
                for c, r in np.argwhere(grid == self.wild_id): mask[c, :], mask[:, r] = True, True
                mask[grid == self.m_ball_id], mask[grid == self.sc_id], mask[grid == self.wild_id] = False, False, True

            if is_win or is_explode:
                res['tumbles'] += 1
                spin_p += p_step
                spin_w += w_step
                grid[mask] = -1
                new_m_map = {}
                for c in range(self.cols):
                    rem = [(grid[c, r], m_values_map.get((c, r))) for r in range(self.rows) if grid[c, r] != -1]
                    needed = self.rows - len(rem)
                    new_e = []
                    if needed > 0:
                        new_ids = [reels[c][(top_indices[c] - i) % len(reels[c])] for i in range(1, needed + 1)][::-1]
                        top_indices[c] = (top_indices[c] - needed) % len(reels[c])
                        for sid in new_ids: new_e.append([sid, None])
                        if self.m_ball_id not in [x[0] for x in rem] and rng.random() < p['m_prob']:
                            v = [i for i, x in enumerate(new_e) if x[0] not in [self.sc_id, self.wild_id]]
                            if v:
                                idx = rng.choice(v)
                                new_e[idx][0], new_e[idx][1] = self.m_ball_id, rng.choice(p['m_vals'], p=p['m_weights'])
                        t_w = rng.choice([int(k) for k in p['wild_refill'].keys()],
                                         p=np.array(list(p['wild_refill'].values())) / sum(p['wild_refill'].values()))
                        for _ in range(min(t_w, self.wild_max - (np.count_nonzero(grid == self.wild_id) + [x[0] for x in new_e].count(self.wild_id)))):
                            v_w = [i for i, x in enumerate(new_e) if x[0] not in [self.sc_id, self.m_ball_id, self.wild_id]]
                            if v_w: new_e[rng.choice(v_w)][0] = self.wild_id

                    for r_idx, (s_id, m_val) in enumerate(new_e + rem):
                        grid[c, r_idx] = s_id
                        if s_id == self.wild_id: res['wild_count'] += 1
                        if s_id == self.m_ball_id:
                            new_m_map[(c, r_idx)] = m_val if m_val is not None else m_values_map.get((c, r_idx), rng.choice(p['m_vals'], p=p['m_weights']))
                m_values_map = new_m_map
            else:
                break

        res['sc_count'] = np.count_nonzero(grid == self.sc_id)
        if mode_key != 'base_game' and (spin_p + spin_w) > 0 and sum(m_values_map.values()) > 0:
            res['win_m_total'] = sum(m_values_map.values())
            f_win = (spin_p + spin_w) * res['win_m_total']
            if f_win / self.base_bet >= self.max_win_cap:
                f_win = self.max_win_cap * self.base_bet
                res['is_max_win'] = True
            rat = spin_p / (spin_p + spin_w) if (spin_p + spin_w) > 0 else 1.0
            res['pure_win'], res['wild_win'] = f_win * rat, f_win * (1 - rat)
        else:
            res['pure_win'], res['wild_win'] = spin_p, spin_w

        res['m_ball_count'] = len(m_values_map)
        return res

    def play_base(self, rng):
        b = self.run_one_spin('base_game', rng)
        sc_pay = self.payouts.get(str(self.sc_id), {}).get(str(min(b['sc_count'], 6)), 0) if b['sc_count'] >= 4 else 0
        res = {'pure_win': b['pure_win'], 'wild_win': b['wild_win'], 'sc_pay': sc_pay, 'fg_win': 0, 'sc': b['sc_count'],
               'fg_trig': False, 'wild_count': b['wild_count'], 'tumbles': b['tumbles'],
               'symbol_contributions': b['symbol_raw_wins'], 'fg_details': None}
        if b['sc_count'] >= 4:
            res['fg_trig'] = True
            res['fg_details'] = self._run_free_logic('free_game', rng, initial_pay=sc_pay)
            res['fg_win'] = res['fg_details']['win']
        return res

    def play_buy(self, rng):
        sc_w = self.m_configs['free_buy']['initial_scatter']
        sc_c = rng.choice([int(k) for k in sc_w.keys()], p=np.array(list(sc_w.values())) / sum(sc_w.values()))
        sc_payout = self.payouts.get(str(self.sc_id), {}).get(str(min(sc_c, 6)), 0)
        fg = self._run_free_logic('free_buy', rng, initial_pay=sc_payout)
        return {'total_win': fg['win'], 'sc_payout': sc_payout, 'sc_trigger': sc_c, 'details': fg}

    def _run_free_logic(self, mode_key, rng, initial_pay=0.0):
        conf = self.m_configs[mode_key]
        spins, total_win, balls, max_hits, total_spins = conf['initial_spins'], initial_pay, 0, 0, 0
        w_win, p_win, w_count, explodes, total_m_sum, fg_win_spins = 0, 0, 0, 0, 0, 0
        fg_combo_list = []
        fg_symbol_wins = collections.defaultdict(float)
        if initial_pay > 0: fg_symbol_wins[self.sc_id] += initial_pay
        max_single_m = 0

        if total_win / self.base_bet >= self.max_win_cap:
            total_win = self.max_win_cap * self.base_bet
            max_hits, spins = 1, 0

        while spins > 0:
            spins -= 1
            total_spins += 1
            f = self.run_one_spin(mode_key, rng)
            this_spin_win = f['pure_win'] + f['wild_win']
            if this_spin_win > 0:
                fg_win_spins += 1
            if (total_win + this_spin_win) / self.base_bet >= self.max_win_cap:
                remaining_win = (self.max_win_cap * self.base_bet) - total_win
                if this_spin_win > 0:
                    ratio = remaining_win / this_spin_win
                    total_win = self.max_win_cap * self.base_bet
                    w_win += f['wild_win'] * ratio
                    p_win += f['pure_win'] * ratio
                    for sid, val in f['symbol_raw_wins'].items(): fg_symbol_wins[sid] += val * ratio
                else: total_win = self.max_win_cap * self.base_bet
                max_hits = 1
                break
            else:
                total_win += this_spin_win
                w_win, p_win = w_win + f['wild_win'], p_win + f['pure_win']
                for sid, val in f['symbol_raw_wins'].items(): fg_symbol_wins[sid] += val
            w_count, explodes, balls = w_count + f['wild_count'], explodes + f['wild_explodes'], balls + f['m_ball_count']
            fg_combo_list.append(f['tumbles'])
            if f['win_m_total'] > 0:
                total_m_sum += f['win_m_total']
                if f['win_m_total'] > max_single_m: max_single_m = f['win_m_total']
            if f['sc_count'] >= conf['retrigger_count']: spins += conf['retrigger_add']

        return {
            'win': total_win, 'w_win': w_win, 'p_win': p_win, 'balls': balls, 'max_hits': max_hits,
            'total_spins': total_spins, 'wild_count': w_count, 'explodes': explodes,
            'm_total_sum': total_m_sum, 'combo_list': fg_combo_list, 'max_single_m': max_single_m,
            'symbol_contributions': fg_symbol_wins,
            'fg_win_spins': fg_win_spins,
        }

def get_combo_range(c):
    if c <= 10: return str(c)
    if c <= 15: return "11-15"
    if c <= 20: return "16-20"
    if c <= 30: return "21-30"
    if c <= 50: return "31-50"
    if c <= 100: return "51-100"
    if c <= 200: return "101-200"
    return "200+"

def sim_task(args):
    num, config, seed, mode = args
    rng = np.random.default_rng(seed)
    sim = SweetWildSimulator(config)
    wild_id = config['game_config']['wild_id']

    m = {
        'base_pw': 0, 'base_ww': 0, 'base_sc': 0,
        'fg_tw': 0, 'fg_pw': 0, 'fg_ww': 0,
        'max_h': 0, 'trigs': 0, 'total_spins': 0,
        'fg_spins_total': 0, 'fg_balls': 0, 'fg_m_sums': [],
        'sc_dist': collections.defaultdict(int),
        'fg_m_dist': collections.defaultdict(int),
        'combo_dist': collections.defaultdict(int),
        'fg_peak_m_dist': collections.defaultdict(int),
        'base_symbol_rtp': collections.defaultdict(float),
        'fg_symbol_rtp': collections.defaultdict(float),
        'base_combo_list': [], 'fg_combo_list': [],
        'base_rounds': 0, 'base_win_times': 0,
        'fg_rounds': 0, 'fg_win_times': 0,
    }

    for _ in range(num):
        if mode == "BASE":
            res = sim.play_base(rng)
            m['total_spins'] += 1
            m['base_rounds'] += 1
            if res['pure_win'] + res['wild_win'] + res['sc_pay'] > 0:
                m['base_win_times'] += 1
            if not res['fg_trig']:
                m['base_pw'] += res['pure_win']
                m['base_ww'] += res['wild_win']
                for sid, val in res['symbol_contributions'].items(): m['base_symbol_rtp'][sid] += val
                m['base_symbol_rtp'][wild_id] += res['wild_win']
                m['base_combo_list'].append(res['tumbles'])
                m['combo_dist'][get_combo_range(res['tumbles'])] += 1
            else:
                m['trigs'] += 1
                m['sc_dist'][res['sc']] += 1
                # 触发局 tumble 线赢（pure/wild）不在 _run_free_logic 的 win 里，须计入 base，与真实派彩及 Go TestRtp 对齐
                m['base_pw'] += res['pure_win']
                m['base_ww'] += res['wild_win']
                for sid, val in res['symbol_contributions'].items():
                    m['base_symbol_rtp'][sid] += val
                m['base_symbol_rtp'][wild_id] += res['wild_win']
                d = res['fg_details']
                m['fg_tw'] += d['win']
                m['fg_pw'] += d['p_win']
                m['fg_ww'] += d['w_win']
                m['fg_balls'] += d['balls']
                m['fg_spins_total'] += d['total_spins']
                m['fg_rounds'] += d['total_spins']
                m['fg_win_times'] += d['fg_win_spins']
                m['max_h'] += d['max_hits']
                m['fg_m_sums'].append(d['m_total_sum'])
                for sid, val in d['symbol_contributions'].items(): m['fg_symbol_rtp'][sid] += val
                m['fg_symbol_rtp'][wild_id] += d['w_win']
                for c_val in d['combo_list']:
                    m['combo_dist'][get_combo_range(c_val)] += 1
                    m['fg_combo_list'].append(c_val)
                # Base触发的那一手也算进Base统计
                m['base_combo_list'].append(res['tumbles'])
                m['combo_dist'][get_combo_range(res['tumbles'])] += 1
                pm = d['max_single_m']
                pk = '0x' if pm == 0 else ('2-10x' if pm <= 10 else ('10-50x' if pm <= 50 else ('50-100x' if pm <= 100 else '100x+')))
                m['fg_peak_m_dist'][pk] += 1
                cur_fg_m = d['m_total_sum']
                if cur_fg_m < 10: m['fg_m_dist']['<10x'] += 1
                elif cur_fg_m < 50: m['fg_m_dist']['10-50x'] += 1
                elif cur_fg_m < 100: m['fg_m_dist']['50-100x'] += 1
                elif cur_fg_m < 500: m['fg_m_dist']['100-500x'] += 1
                else: m['fg_m_dist']['500x+'] += 1
        else:
            res = sim.play_buy(rng)
            m['trigs'] += 1
            d = res['details']
            m['sc_dist'][res['sc_trigger']] += 1
            m['fg_tw'], m['fg_pw'], m['fg_ww'] = m['fg_tw'] + d['win'], m['fg_pw'] + d['p_win'], m['fg_ww'] + d['w_win']
            m['fg_balls'], m['fg_spins_total'], m['max_h'] = m['fg_balls'] + d['balls'], m['fg_spins_total'] + d['total_spins'], m['max_h'] + d['max_hits']
            m['fg_rounds'] += d['total_spins']
            m['fg_win_times'] += d['fg_win_spins']
            m['total_spins'] += d['total_spins']
            m['fg_m_sums'].append(d['m_total_sum'])
            for sid, val in d['symbol_contributions'].items(): m['fg_symbol_rtp'][sid] += val
            m['fg_symbol_rtp'][wild_id] += d['w_win']
            for c_val in d['combo_list']:
                m['combo_dist'][get_combo_range(c_val)] += 1
                m['fg_combo_list'].append(c_val)
            pm = d['max_single_m']
            pk = '0x' if pm == 0 else ('2-10x' if pm <= 10 else ('10-50x' if pm <= 50 else ('50-100x' if pm <= 100 else '100x+')))
            m['fg_peak_m_dist'][pk] += 1
            cur_fg_m = d['m_total_sum']
            if cur_fg_m < 10: m['fg_m_dist']['<10x'] += 1
            elif cur_fg_m < 50: m['fg_m_dist']['10-50x'] += 1
            elif cur_fg_m < 100: m['fg_m_dist']['50-100x'] += 1
            elif cur_fg_m < 500: m['fg_m_dist']['100-500x'] += 1
            else: m['fg_m_dist']['500x+'] += 1
    return m

if __name__ == "__main__":
    with open('config.json', 'r') as f: config = json.load(f)
    print(f"🍭 模式: {SIM_MODE} | 总次数: {TOTAL_COUNT} | 线程: {cpu_count()}")
    with Pool(cpu_count()) as p:
        raw = list(tqdm(p.imap_unordered(sim_task, [(TOTAL_COUNT // 1000, config, s, SIM_MODE) for s in np.random.SeedSequence().spawn(1000)]), total=1000))

    def sum_key(k): return sum(r[k] for r in raw)

    b_total = sum_key('base_pw') + sum_key('base_ww') + sum_key('base_sc')
    f_total, t_trig, max_win_hits = sum_key('fg_tw'), max(1, sum_key('trigs')), sum_key('max_h')
    total_win = b_total + f_total
    cost = TOTAL_COUNT * config['game_config']['base_bet'] * (100 if SIM_MODE == "BUY" else 1)

    base_rtp_map, fg_rtp_map = collections.defaultdict(float), collections.defaultdict(float)
    combo_t, peak_dist, m_dist = collections.defaultdict(int), collections.defaultdict(int), collections.defaultdict(int)
    all_ms, all_base_c, all_fg_c = [], [], []

    for r in raw:
        all_ms.extend(r['fg_m_sums'])
        all_base_c.extend(r['base_combo_list'])
        all_fg_c.extend(r['fg_combo_list'])
        for k, v in r['base_symbol_rtp'].items(): base_rtp_map[k] += v
        for k, v in r['fg_symbol_rtp'].items(): fg_rtp_map[k] += v
        for k, v in r['combo_dist'].items(): combo_t[k] += v
        for k, v in r['fg_peak_m_dist'].items(): peak_dist[k] += v
        for k, v in r['fg_m_dist'].items(): m_dist[k] += v

    print("\n" + "═" * 75)
    print(f"║{'SWEET WILD 深度仿真报告 (聚合统计版)':^65}║")
    print("═" * 75)
    print(f"【整体经济指标】")
    print(f"  > 总返还 (RTP)  :  {total_win / cost * 100:>10.2f} %")
    print(f"  > Base 贡献 RTP :  {b_total / cost * 100:>10.2f} %")
    print(f"  > Free 贡献 RTP :  {f_total / cost * 100:>10.2f} %")
    print(f"  > FG 触发频率   : 1 / {(TOTAL_COUNT / t_trig):.1f} 转")
    base_rounds, base_win_times = sum_key('base_rounds'), sum_key('base_win_times')
    fg_rounds, fg_win_times = sum_key('fg_rounds'), sum_key('fg_win_times')
    print(f"  > Base 中奖率   :  {base_win_times / base_rounds * 100 if base_rounds else 0:>10.4f} %  ({base_win_times}/{base_rounds})")
    print(f"  > Free 中奖率   :  {fg_win_times / fg_rounds * 100 if fg_rounds else 0:>10.4f} %  ({fg_win_times}/{fg_rounds})")
    print(f"  > MaxWin 触发数 :  {max_win_hits:<10} (1 / {TOTAL_COUNT / (max_win_hits if max_win_hits > 0 else 1):.0f} 转)")
    print("-" * 75)
    print(f"【图标贡献占比详情】")
    print(f"  {'图标 ID':<10} | {'Base RTP %':<12} | {'Free RTP %':<12} | {'总 RTP %':<10}")
    print("-" * 75)
    wild_id, sc_id = config['game_config']['wild_id'], config['game_config']['sc_id']
    for sid in sorted(set(list(base_rtp_map.keys()) + list(fg_rtp_map.keys()))):
        br, fr = (base_rtp_map[sid] / cost) * 100, (fg_rtp_map[sid] / cost) * 100
        name = f"WILD ({sid})" if sid == wild_id else (f"SC ({sid})" if sid == sc_id else f"ID {sid}")
        print(f"  {name:<10} | {br:>11.2f}% | {fr:>11.2f}% | {br + fr:>9.2f}%")

    print("-" * 75)
    print(f"【连击深度分布 (区间聚合)】")
    combo_order = [str(i) for i in range(11)] + ["11-15", "16-20", "21-30", "31-50", "51-100", "101-200", "200+"]
    total_combo_events = sum(combo_t.values())
    for k in combo_order:
        if combo_t[k] == 0: continue
        perc = combo_t[k] / (total_combo_events if total_combo_events > 0 else 1) * 100
        print(f"  消除 {k:<8} 次 : {perc:>6.2f}% {'█' * int(perc / 5)}")

    print("-" * 75)
    print(f"【消除强度统计】")
    print(f"  > Base 模式 : 平均 {np.mean(all_base_c) if all_base_c else 0:>5.2f} 次 | 最大 {np.max(all_base_c) if all_base_c else 0:>3} 次")
    print(f"  > Free 模式 : 平均 {np.mean(all_fg_c) if all_fg_c else 0:>5.2f} 次 | 最大 {np.max(all_fg_c) if all_fg_c else 0:>3} 次")

    print("-" * 75)
    print(f"【FG单场巅峰倍数分布】")
    for k in ['0x', '2-10x', '10-50x', '50-100x', '100x+']:
        perc = peak_dist[k] / t_trig * 100
        print(f"  巅峰 {k:<8} : {perc:>6.2f}% {'█' * int(perc / 5)}")
    print("-" * 75)
    print(f"【FREE 模式构成】")
    print(f"  > FG 场均旋转数 : {sum_key('fg_spins_total') / t_trig:>10.2f} 转")
    print(f"  > FG 场均连击倍数: {np.mean(all_ms) if all_ms else 0:>10.2f} x")
    print(f"  > FG 场均掉球    : {sum_key('fg_balls') / t_trig:>10.2f} 个")
    print("-" * 75)
    print(f"【FREE GAME 累计倍数分布】")
    for k in ['<10x', '10-50x', '50-100x', '100-500x', '500x+']:
        perc = m_dist[k] / t_trig * 100
        print(f"  {k:<10} : {perc:>6.2f}% {'█' * int(perc / 5)}")
    print("-" * 75)
    sc_t = collections.defaultdict(int)
    for r in raw:
        for k, v in r['sc_dist'].items(): sc_t[k] += v
    print(f"【Scatter 进场分布】")
    for i in range(4, 9):
        if sc_t[i] > 0: print(f"  SC {i} 触发    : {sc_t[i] / t_trig * 100:>6.2f}%")
    print("═" * 75)

